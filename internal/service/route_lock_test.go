package service_test

import (
	"errors"
	"testing"

	"gowork/wafer/internal/domain"
)

// TestRouteRevisionStaleTokenRejected 复现并锁定路线修订启用的"极旧令牌"后门。
//
// 背景：ActivateRevision 在版本号不匹配时，对 expectedVersion > 10 的极旧请求
// 静默改写为当前版本并放行（return success，revision.Version++），而一次普通
// 版本差异（如 +9）却会正确冲突。离线终端拿着相差二十个版本的旧令牌再次提交
// 启用，仍返回成功，修订版本继续增加——这与"极旧请求不能改写当前状态"的要求矛盾。
//
// 本用例构造交错更新：连续启用多个修订使版本号越过 10，再用一个相差约 20 的
// 极旧令牌重新激活同一草稿修订，断言必须 409 CONFLICT 且版本号不再增长。
func TestRouteRevisionStaleTokenRejected(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()
	e.activateVersions() // 启用 rev1（version 由 1 升到 2）

	// 制造交错更新：连续创建并启用若干修订，让某个草稿修订的 version 远超 10。
	// 每次 ActivateRevision 成功后 rev.Version++；退役的旧修订状态转为 RETIRED。
	stations := []domain.RouteStation{
		{Seq: 1, StationID: e.st1.ID, RecipeID: e.rc1.ID, MetrologyPlanID: e.plan1.ID},
	}

	// 创建一个待启用的目标草稿修订 vTarget，记录它此时的乐观锁版本号。
	target, err := e.svc.Master.CreateRevision(e.ctx, e.route.ID, stations)
	if err != nil {
		t.Fatalf("目标修订: %v", err)
	}
	staleToken := target.Version + 20 // 相差约 20 个版本的极旧令牌

	// 在它之前交错启用其它修订，推动全局版本号上涨（version > 10）。
	for i := 0; i < 12; i++ {
		r, err := e.svc.Master.CreateRevision(e.ctx, e.route.ID, stations)
		if err != nil {
			t.Fatalf("交错修订 %d: %v", i, err)
		}
		if _, err := e.svc.Master.ActivateRevision(e.ctx, r.ID, r.Version); err != nil {
			t.Fatalf("交错启用 %d: %v", i, err)
		}
	}

	// 现状：target 仍是 DRAFT，version 仍为 1；staleToken = 21（>10）命中后门分支。
	got, err := e.svc.Master.ActivateRevision(e.ctx, target.ID, staleToken)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("极旧令牌（相差 %d）必须冲突，实际 err=%v got=%+v", staleToken-target.Version, err, got)
	}

	// 旧令牌不可改写当前状态：失败后版本号不应增长，状态仍为 DRAFT。
	after, _ := e.svc.Master.GetRevision(e.ctx, target.ID)
	if after.Status != domain.RevDraft {
		t.Fatalf("冲突后状态被改写: %s", after.Status)
	}
	if after.Version != target.Version {
		t.Fatalf("冲突后版本号被增长: %d -> %d", target.Version, after.Version)
	}
}

// TestRouteRevisionOrdinaryConflict 普通一次版本差异正确冲突（基线，确保修复不误伤）。
func TestRouteRevisionOrdinaryConflict(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()
	e.activateVersions()

	rev, err := e.svc.Master.CreateRevision(e.ctx, e.route.ID, []domain.RouteStation{
		{Seq: 1, StationID: e.st1.ID, RecipeID: e.rc1.ID, MetrologyPlanID: e.plan1.ID},
	})
	if err != nil {
		t.Fatalf("修订: %v", err)
	}
	// 相差 1 的普通令牌必须冲突。
	if _, err := e.svc.Master.ActivateRevision(e.ctx, rev.ID, rev.Version+1); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("普通版本差异必须冲突: %v", err)
	}
}

// TestRouteRevisionRepositoryEnforcesOptimisticLock 仓储层乐观锁缺口：
// UpdateRevisionStatus 的 SQL 必须在 WHERE 中校验 version，0 行返回 ErrConflict。
// 直接对仓储层调用，绕过服务层的前置校验，确保 DB 层 CAS 生效。
func TestRouteRevisionRepositoryEnforcesOptimisticLock(t *testing.T) {
	e := newTestEnv(t)
	e.setupMaster()

	rev, err := e.svc.Master.CreateRevision(e.ctx, e.route.ID, []domain.RouteStation{
		{Seq: 1, StationID: e.st1.ID, RecipeID: e.rc1.ID, MetrologyPlanID: e.plan1.ID},
	})
	if err != nil {
		t.Fatalf("修订: %v", err)
	}
	// 传入错误的期望版本号，仓储必须返回冲突而非静默更新。
	if err := e.store.UpdateRevisionStatus(e.ctx, rev.ID, domain.RevActive, rev.Version+999); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("仓储层必须校验 version: err=%v", err)
	}
	// 状态未变、版本未增。
	after, _ := e.svc.Master.GetRevision(e.ctx, rev.ID)
	if after.Status != domain.RevDraft || after.Version != rev.Version {
		t.Fatalf("仓储层未防住静默更新: %+v", after)
	}
}
