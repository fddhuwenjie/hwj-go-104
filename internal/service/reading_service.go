package service

import (
	"context"
	"encoding/json"
	"fmt"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/rules"
)

// ReadingService 量测服务：读数采集、封存、自动判定与暂扣生成。
type ReadingService struct {
	d Deps
}

// ReadingInput 读数提交输入。
type ReadingInput struct {
	WaferID string  `json:"wafer_id"`
	Metric  string  `json:"metric"`
	Value   float64 `json:"value"`
}

// SubmitReadings 提交量测读数：仅允许运行完工后提交；
// 读数的晶圆必须实际参与目标运行（属于该运行的 run_wafers），
// 否则按晶圆与运行追溯会得到矛盾结果，一律拒绝；
// 运行已判定时作为迟到量测附着原运行，不覆盖当前有效判定（幂等）。
func (s *ReadingService) SubmitReadings(ctx context.Context, runID string, inputs []ReadingInput, idemKey string) ([]domain.Reading, bool, error) {
	return idempotent(ctx, s.d, "reading.submit:"+runID, idemKey, func(tx context.Context) ([]domain.Reading, error) {
		run, err := s.d.Store.GetRun(tx, runID)
		if err != nil {
			return nil, err
		}
		if run.Status == domain.RunRunning || run.Status == domain.RunAborted {
			return nil, domain.ErrRunNotCompleted
		}
		late := run.Status == domain.RunJudged
		if len(inputs) == 0 {
			return nil, domain.NewValidationError(domain.FieldError{Field: "readings", Message: "读数不能为空"})
		}
		runWafers, err := s.d.Store.RunWafers(tx, runID)
		if err != nil {
			return nil, err
		}
		// 读数晶圆必须实际参与目标运行。仅校验晶圆状态不足以排除跨运行混入：
		// 任何已登记活跃晶圆都属于某次运行，但读数只能挂在它真正参与的运行上。
		member := map[string]bool{}
		for _, id := range runWafers {
			member[id] = true
		}
		// 批次内重复输入须去重，避免同一晶圆写多条读数。
		seen := map[string]bool{}
		var out []domain.Reading
		for _, in := range inputs {
			if !member[in.WaferID] {
				return nil, fmt.Errorf("%w: 晶圆 %s 未参与运行 %s", domain.ErrValidation, in.WaferID, runID)
			}
			if seen[in.WaferID] {
				return nil, fmt.Errorf("%w: 晶圆 %s 重复提交", domain.ErrValidation, in.WaferID)
			}
			seen[in.WaferID] = true
			w, err := s.d.Store.GetWafer(tx, in.WaferID)
			if err != nil {
				return nil, err
			}
			r := domain.Reading{
				ID:        domain.NewID(domain.IDPrefixReading),
				RunID:     runID,
				WaferID:   in.WaferID,
				Slot:      w.Slot,
				Metric:    in.Metric,
				Value:     in.Value,
				Late:      late,
				Sealed:    false,
				CreatedAt: s.d.Clock.Now(),
			}
			if err := r.Validate(); err != nil {
				return nil, err
			}
			if err := s.d.Store.CreateReading(tx, &r); err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, nil
	})
}

// ListReadings 列出运行读数。
func (s *ReadingService) ListReadings(ctx context.Context, runID string) ([]domain.Reading, error) {
	return s.d.Store.ListReadings(ctx, runID)
}

// Seal 量测封存：校验抽样覆盖（计划位置与最小数量）、自动判定、
// 判定失败生成暂扣并置批次 ON_HOLD，全部在同一事务内（幂等、不可逆）。
func (s *ReadingService) Seal(ctx context.Context, runID string, idemKey string) (*domain.Run, bool, error) {
	return idempotent(ctx, s.d, "reading.seal:"+runID, idemKey, func(tx context.Context) (*domain.Run, error) {
		run, err := s.d.Store.GetRun(tx, runID)
		if err != nil {
			return nil, err
		}
		if run.Status != domain.RunCompleted {
			return nil, fmt.Errorf("%w: 运行状态 %s 不可封存", domain.ErrInvalidState, run.Status)
		}
		lot, err := s.d.Store.GetLot(tx, run.LotID)
		if err != nil {
			return nil, err
		}
		snap, err := loadFreeze(lot)
		if err != nil {
			return nil, err
		}
		fs := snap.StationAt(run.StationSeq)
		if fs == nil || run.RouteRevisionID != lot.FrozenRevisionID {
			return nil, fmt.Errorf("%w: 运行与批次冻结快照不一致", domain.ErrInvalidState)
		}
		var plan domain.MetrologyPlan
		if err := json.Unmarshal([]byte(fs.PlanSnapshot), &plan); err != nil {
			return nil, fmt.Errorf("量测计划快照损坏: %w", err)
		}
		readings, err := s.d.Store.ListReadings(tx, runID)
		if err != nil {
			return nil, err
		}
		lotWafers, err := s.d.Store.ListWafers(tx, lot.ID)
		if err != nil {
			return nil, err
		}
		slots := map[int]bool{}
		for _, w := range lotWafers {
			slots[w.Slot] = true
		}
		if err := rules.CheckSamplingCoverage(plan, readings, slots); err != nil {
			return nil, err
		}
		judgment := rules.AutoJudge(plan, readings)
		if judgment == domain.JudgeNone {
			return nil, domain.ErrSampling
		}
		if err := s.d.Store.SealReadings(tx, runID); err != nil {
			return nil, err
		}
		run.Status = domain.RunJudged
		run.Judgment = judgment
		if err := s.d.Store.UpdateRun(tx, run, run.Version); err != nil {
			return nil, err
		}
		run.Version++
		txTag := domain.NewID("tx")
		if err := audit(tx, s.d, txTag, "run", run.ID, domain.AuditReadingSeal,
			map[string]any{"judgment": judgment, "readings": len(readings)}); err != nil {
			return nil, err
		}
		if judgment == domain.JudgeFail {
			hold := &domain.Hold{
				ID:        domain.NewID(domain.IDPrefixHold),
				LotID:     lot.ID,
				RunID:     run.ID,
				Reason:    "量测自动判定失败",
				Status:    domain.HoldOpen,
				CreatedAt: s.d.Clock.Now(),
			}
			if err := s.d.Store.CreateHold(tx, hold); err != nil {
				return nil, err
			}
			lot.Status = domain.LotOnHold
			if err := s.d.Store.UpdateLot(tx, lot, lot.Version); err != nil {
				return nil, err
			}
			if err := audit(tx, s.d, txTag, "hold", hold.ID, domain.AuditHoldCreate, hold); err != nil {
				return nil, err
			}
		}
		return run, nil
	})
}
