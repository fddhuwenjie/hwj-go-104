package sqlite

import (
	"context"
	"database/sql"
	"time"

	"gowork/wafer/internal/domain"
	"gowork/wafer/internal/repository"
)

// ExpiredQualificationRuns 过期资质但尚未复判的运行，稳定分页。
func (s *Store) ExpiredQualificationRuns(ctx context.Context, page domain.Page) ([]repository.ExpiredQualificationRun, error) {
	page = page.Normalize()
	key, err := domain.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	query := `SELECT r.id, r.lot_id, l.code, r.station_seq, r.equipment_id, r.chamber_id,
	    r.started_at, r.completed_at, r.created_at
	  FROM runs r JOIN lots l ON l.id = r.lot_id
	  WHERE r.qual_covered=0 AND r.reviewed=0 AND r.status IN (?,?)`
	args := []any{domain.RunCompleted, domain.RunJudged}
	if page.Cursor != "" {
		query += ` AND ((r.created_at > ?) OR (r.created_at = ? AND r.id > ?))`
		args = append(args, ms(key.CreatedAt), ms(key.CreatedAt), key.ID)
	}
	query += ` ORDER BY r.created_at, r.id DESC LIMIT ?`
	args = append(args, page.Limit)
	rows, err := s.q(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repository.ExpiredQualificationRun
	for rows.Next() {
		var v repository.ExpiredQualificationRun
		var started, completed, created int64
		if err := rows.Scan(&v.RunID, &v.LotID, &v.LotCode, &v.StationSeq, &v.EquipmentID,
			&v.ChamberID, &started, &completed, &created); err != nil {
			return nil, err
		}
		v.StartedAt, v.CompletedAt, v.CreatedAt = tm(started), tm(completed), tm(created)
		out = append(out, v)
	}
	return out, rows.Err()
}

// WipLots 当前在制批次（含冻结修订号与最近暂扣原因），稳定分页。
func (s *Store) WipLots(ctx context.Context, page domain.Page) ([]repository.WipLot, error) {
	page = page.Normalize()
	key, err := domain.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	query := `SELECT l.id, l.code, l.status, l.current_seq, rr.revision, l.frozen_at,
	    (SELECT h.reason FROM holds h WHERE h.lot_id=l.id ORDER BY h.created_at DESC, h.id DESC LIMIT 1),
	    l.created_at
	  FROM lots l
	  LEFT JOIN route_revisions rr ON rr.id = l.frozen_revision_id
	  WHERE l.status IN (?,?,?,?,?)`
	args := []any{domain.LotRegistered, domain.LotQueued, domain.LotRunning, domain.LotWaiting, domain.LotOnHold}
	if page.Cursor != "" {
		query += ` AND ((l.created_at > ?) OR (l.created_at = ? AND l.id > ?))`
		args = append(args, ms(key.CreatedAt), ms(key.CreatedAt), key.ID)
	}
	query += ` ORDER BY l.created_at, l.id LIMIT ?`
	args = append(args, page.Limit)
	rows, err := s.q(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repository.WipLot
	for rows.Next() {
		var v repository.WipLot
		var rev sql.NullInt64
		var frozenAt sql.NullInt64
		var holdReason sql.NullString
		var created int64
		if err := rows.Scan(&v.LotID, &v.Code, &v.Status, &v.CurrentSeq, &rev, &frozenAt, &holdReason, &created); err != nil {
			return nil, err
		}
		if rev.Valid {
			n := int(rev.Int64)
			v.FrozenRevision = &n
		}
		v.FrozenAt = tmPtr(frozenAt)
		if holdReason.Valid {
			v.LatestHoldReason = holdReason.String
		}
		v.CreatedAt = tm(created)
		out = append(out, v)
	}
	return out, rows.Err()
}

// StationQueues 等待时间超过阈值且设备能力可用的站点队列。
// 在 QUEUED 状态的批次等待其 current_seq 站点；设备能力可用定义为：
// 站点下存在 ACTIVE 设备且其至少一个 ACTIVE 腔体能力覆盖站点要求。
func (s *Store) StationQueues(ctx context.Context, minWait time.Duration, now time.Time) ([]repository.StationQueueItem, error) {
	deadline := now.Add(-minWait)
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT l.id, l.code, l.current_seq, l.entered_at, l.freeze_snapshot
		 FROM lots l WHERE l.status=? AND l.entered_at IS NOT NULL AND l.entered_at<=?
		 ORDER BY l.entered_at, l.id`, domain.LotQueued, ms(deadline))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type queuedLot struct {
		id, code  string
		seq       int
		enteredAt int64
		snapRaw   string
	}
	var queued []queuedLot
	for rows.Next() {
		var q queuedLot
		if err := rows.Scan(&q.id, &q.code, &q.seq, &q.enteredAt, &q.snapRaw); err != nil {
			return nil, err
		}
		queued = append(queued, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []repository.StationQueueItem
	for _, q := range queued {
		snap, err := domain.DecodeFreezeSnapshot(q.snapRaw)
		if err != nil {
			continue
		}
		fs := snap.StationAt(q.seq)
		if fs == nil {
			continue
		}
		capable, err := s.countCapableEquipment(ctx, fs.StationID, fs.Capability)
		if err != nil {
			return nil, err
		}
		if capable == 0 {
			continue
		}
		entered := tm(q.enteredAt)
		out = append(out, repository.StationQueueItem{
			StationID:        fs.StationID,
			StationCode:      fs.StationCode,
			LotID:            q.id,
			LotCode:          q.code,
			QueuedAt:         entered,
			WaitSeconds:      int64(now.Sub(entered).Seconds()),
			CapableEquipment: capable,
		})
	}
	return out, nil
}

// countCapableEquipment 统计站点下能力可用的 ACTIVE 设备数量。
func (s *Store) countCapableEquipment(ctx context.Context, stationID, capability string) (int, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT DISTINCT e.id, c.capability FROM equipment e
		 JOIN chambers c ON c.equipment_id = e.id AND c.status='ACTIVE'
		 WHERE e.station_id=? AND e.status='ACTIVE'`, stationID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var id, cap string
		if err := rows.Scan(&id, &cap); err != nil {
			return 0, err
		}
		if capabilityCovers(cap, capability) {
			seen[id] = true
		}
	}
	return len(seen), rows.Err()
}

// capabilityCovers 与 rules 层一致的标签覆盖判断（查询层本地副本避免 SQL 复杂化）。
func capabilityCovers(have, required string) bool {
	set := map[string]bool{}
	for _, t := range splitTags(have) {
		set[t] = true
	}
	for _, t := range splitTags(required) {
		if !set[t] {
			return false
		}
	}
	return true
}

func splitTags(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			tag := trimSpace(s[start:i])
			if tag != "" {
				out = append(out, tag)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// ReworkStats 按设备腔体与配方版本聚合的重复返工批次。
// 重复返工定义：批次存在两条及以上返工记录。
func (s *Store) ReworkStats(ctx context.Context) ([]repository.ReworkStat, error) {
	rows, err := s.q(ctx).QueryContext(ctx,
		`WITH repeat_lots AS (
		   SELECT lot_id FROM rework_records GROUP BY lot_id HAVING COUNT(*) >= 2
		 )
		 SELECT r.equipment_id, r.chamber_id, r.recipe_version_id, COUNT(DISTINCT h.lot_id)
		 FROM holds h
		 JOIN runs r ON r.id = h.run_id
		 WHERE h.status=? AND h.lot_id IN (SELECT lot_id FROM repeat_lots)
		 GROUP BY r.equipment_id, r.chamber_id, r.recipe_version_id
		 ORDER BY r.equipment_id, r.chamber_id, r.recipe_version_id`, domain.HoldReworked)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repository.ReworkStat
	for rows.Next() {
		var v repository.ReworkStat
		if err := rows.Scan(&v.EquipmentID, &v.ChamberID, &v.RecipeVersionID, &v.ReworkLots); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GenealogyAudit 父子批不一致审计：
// STATUS_MISMATCH：父批已报废/关闭但子批仍在制；
// WAFER_NO_MOVE：晶圆归属子批但缺少迁移记录（谱系断裂）。
func (s *Store) GenealogyAudit(ctx context.Context) ([]repository.GenealogyIssue, error) {
	var out []repository.GenealogyIssue
	rows, err := s.q(ctx).QueryContext(ctx,
		`SELECT p.id, c.id, p.status, c.status FROM lots p
		 JOIN lots c ON c.parent_lot_id = p.id
		 WHERE p.status IN (?,?) AND c.status NOT IN (?,?)`,
		domain.LotScrapped, domain.LotClosed, domain.LotScrapped, domain.LotClosed)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var pid, cid, ps, cs string
		if err := rows.Scan(&pid, &cid, &ps, &cs); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, repository.GenealogyIssue{
			Issue:   "STATUS_MISMATCH",
			LotID:   cid,
			Related: pid,
			Detail:  "父批状态 " + ps + " 与子批状态 " + cs + " 不一致",
		})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows2, err := s.q(ctx).QueryContext(ctx,
		`SELECT w.id, w.lot_id FROM wafers w
		 JOIN lots c ON c.id = w.lot_id
		 WHERE c.parent_lot_id <> ''
		   AND NOT EXISTS (SELECT 1 FROM wafer_moves m WHERE m.wafer_id = w.id AND m.to_lot_id = w.lot_id)`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var wid, lotID string
		if err := rows2.Scan(&wid, &lotID); err != nil {
			return nil, err
		}
		out = append(out, repository.GenealogyIssue{
			Issue:   "WAFER_NO_MOVE",
			LotID:   lotID,
			Related: wid,
			Detail:  "晶圆归属子批但缺少迁移记录，谱系断裂",
		})
	}
	return out, rows2.Err()
}
