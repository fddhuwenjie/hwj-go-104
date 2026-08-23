package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
)

// TestMetrologyPlanStaleActivateRejected 复现并验证量测计划乐观锁：
// 两名工艺工程师同时打开同一量测计划草稿，一人先保存并启用后，
// 另一人仍拿旧版本（stale row_version）提交也显示成功——陈旧操作本来必须冲突，
// 且不能改变当前计划状态与版本。
//
// 修复前：ActivatePlan 用旧 RowVersion 仍返回成功，row_version 继续增长。
// 修复后：陈旧提交必须返回 ErrConflict，计划状态/版本保持不变。
func TestMetrologyPlanStaleActivateRejected(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()

	ctx := e.ctx
	m := e.svc.Master

	// 两名工程师同时打开同一份草稿，各自读到 row_version=1。
	draft, err := m.CreatePlan(ctx, "MP-COLLIDE", "协同编辑量测", "cd", []int{1, 2}, 2, 10.0)
	if err != nil {
		t.Fatalf("建档草稿: %v", err)
	}
	staleVersion := draft.RowVersion // 工程师 B 持有的旧版本

	// 工程师 A 先保存并启用，计划转为 ACTIVE，row_version 增长到 2。
	activated, err := m.ActivatePlan(ctx, draft.ID, draft.RowVersion)
	if err != nil {
		t.Fatalf("工程师 A 启用: %v", err)
	}
	if activated.Status != domain.PlanActive {
		t.Fatalf("启用后状态错误: %s", activated.Status)
	}
	currentVersion := activated.RowVersion

	// 工程师 B 仍拿旧版本提交启用——交错提交必须冲突，不能穿透乐观锁。
	_, err = m.ActivatePlan(ctx, draft.ID, staleVersion)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("陈旧版本提交必须冲突: %v", err)
	}

	// 冲突不得改变当前计划状态与版本：仍为 ACTIVE 且版本未继续增长。
	after, err := m.GetPlan(ctx, draft.ID)
	if err != nil {
		t.Fatalf("读取计划: %v", err)
	}
	if after.Status != domain.PlanActive {
		t.Fatalf("冲突提交后计划状态被改变: %s", after.Status)
	}
	if after.RowVersion != currentVersion {
		t.Fatalf("冲突提交后版本继续增长: got %d want %d", after.RowVersion, currentVersion)
	}
}

// TestMetrologyPlanStoreLayerOptimisticLock 验证仓储层乐观锁：
// 直接对同一计划用陈旧 row_version 调用 UpdatePlanStatus 必须返回 ErrConflict，
// 而不是无条件更新成功（修复前 SQL WHERE 子句缺少 row_version 匹配）。
func TestMetrologyPlanStoreLayerOptimisticLock(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()

	ctx := e.ctx
	m := e.svc.Master

	draft, err := m.CreatePlan(ctx, "MP-STORELOCK", "仓储锁量测", "cd", []int{1}, 1, 8.0)
	if err != nil {
		t.Fatalf("建档草稿: %v", err)
	}
	staleVersion := draft.RowVersion

	// 先用正确版本退役一次，row_version 增长。
	if err := e.store.UpdatePlanStatus(ctx, draft.ID, domain.PlanRetired, draft.RowVersion); err != nil {
		t.Fatalf("首次状态更新: %v", err)
	}

	// 再用陈旧版本更新必须冲突。
	if err := e.store.UpdatePlanStatus(ctx, draft.ID, domain.PlanActive, staleVersion); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("仓储层陈旧版本必须冲突: %v", err)
	}

	// 计划状态保持退役，版本未被陈旧提交再次增长。
	after, err := m.GetPlan(ctx, draft.ID)
	if err != nil {
		t.Fatalf("读取计划: %v", err)
	}
	if after.Status != domain.PlanRetired {
		t.Fatalf("陈旧提交改变了状态: %s", after.Status)
	}
}
