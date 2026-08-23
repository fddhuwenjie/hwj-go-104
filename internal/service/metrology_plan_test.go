package service_test

import (
	"context"
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
	"gowork/wafer/internal/service"
)

// failingAuditStore 包装真实存储，使审计写入可控失败，用于复现事务边界缺陷。
type failingAuditStore struct {
	repository.Store
	auditErr error
}

func (f *failingAuditStore) CreateAudit(ctx context.Context, e *domain.AuditEvent) error {
	if f.auditErr != nil {
		return f.auditErr
	}
	return f.Store.CreateAudit(ctx, e)
}

// TestActivatePlanAtomicOnAuditFailure 复现"启用同编码新计划时审计写入失败"的部分切换：
// 退役旧计划、启用新计划、写入审计必须整体提交或整体回滚。
// 当审计写入失败时，旧计划必须保持启用、新计划必须保持草稿——
// 不得出现"旧计划已退役、新计划仍草稿"的断裂（生产线将找不到有效计划）。
func TestActivatePlanAtomicOnAuditFailure(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster() // plan1(MP-ETCH) 与 plan2 已启用

	// 同编码 MP-ETCH 新建草稿版本 v2。
	newPlan, err := e.svc.Master.CreatePlan(e.ctx, "MP-ETCH", "刻蚀量测v2", "cd", []int{1, 2}, 2, 10.0)
	if err != nil {
		t.Fatalf("新建计划v2: %v", err)
	}

	// 注入审计写入故障，复现部分切换。
	svc := service.NewServices(service.Deps{
		Store: &failingAuditStore{Store: e.store, auditErr: errors.New("audit write failure")},
		Clock: e.clk,
	})
	if _, err := svc.Master.ActivatePlan(e.ctx, newPlan.ID, newPlan.RowVersion); err == nil {
		t.Fatal("审计写入失败时启用计划必须返回错误")
	}

	// 整体回滚：旧计划仍启用、新计划仍草稿。
	old, err := e.svc.Master.GetPlan(e.ctx, e.plan1.ID)
	if err != nil {
		t.Fatalf("查询旧计划: %v", err)
	}
	if old.Status != domain.PlanActive {
		t.Fatalf("审计失败后旧计划应保持启用，实际 %s（旧计划已被退役 → 部分切换）", old.Status)
	}
	cur, err := e.svc.Master.GetPlan(e.ctx, newPlan.ID)
	if err != nil {
		t.Fatalf("查询新计划: %v", err)
	}
	if cur.Status != domain.PlanDraft {
		t.Fatalf("审计失败后新计划应保持草稿，实际 %s", cur.Status)
	}

	// 审计事件不应残留：整笔事务回滚则无 activate 审计。
	audits, err := e.svc.Query.ListAudit(e.ctx, "metrology_plan", newPlan.ID)
	if err != nil {
		t.Fatalf("查询审计: %v", err)
	}
	if len(audits) != 0 {
		t.Fatalf("回滚后不应残留 activate 审计，实际 %d 条", len(audits))
	}
}

// TestActivatePlanSwapsSameCode 整体提交的正向路径：启用同编码新版本时，
// 旧版本退役、新版本启用、审计事件写入在同一事务内成功落地。
func TestActivatePlanSwapsSameCode(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster() // plan1(MP-ETCH) 已启用

	newPlan, err := e.svc.Master.CreatePlan(e.ctx, "MP-ETCH", "刻蚀量测v2", "cd", []int{1, 2}, 2, 10.0)
	if err != nil {
		t.Fatalf("新建计划v2: %v", err)
	}
	activated, err := e.svc.Master.ActivatePlan(e.ctx, newPlan.ID, newPlan.RowVersion)
	if err != nil {
		t.Fatalf("启用新计划: %v", err)
	}
	if activated.Status != domain.PlanActive || activated.RowVersion != newPlan.RowVersion+1 {
		t.Fatalf("新计划启用状态错误: %+v", activated)
	}
	old, _ := e.svc.Master.GetPlan(e.ctx, e.plan1.ID)
	if old.Status != domain.PlanRetired {
		t.Fatalf("旧计划应退役，实际 %s", old.Status)
	}
	// 同编码恰好一个启用计划，不致生产线找不到有效计划。
	plans, _ := e.svc.Master.ListPlans(e.ctx)
	active := 0
	for _, p := range plans {
		if p.Code == "MP-ETCH" && p.Status == domain.PlanActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("同编码应有且仅有一个启用计划，实际 %d", active)
	}
	audits, _ := e.svc.Query.ListAudit(e.ctx, "metrology_plan", newPlan.ID)
	if len(audits) != 1 || audits[0].Action != "metrology_plan.activate" {
		t.Fatalf("启用审计事件缺失或错误: %+v", audits)
	}
}
