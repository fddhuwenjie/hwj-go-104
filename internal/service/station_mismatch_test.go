package service_test

import (
	"errors"
	"testing"
	"time"

	"gowork/wafer/internal/domain"
)

// TestStartRejectsEquipmentFromOtherStation 复现“设备所属站点与当前工序矛盾被双重掩盖”。
//
// 场景：批次当前位于刻蚀站点(st1)，排产选中挂在清洗站点(st2)的设备 eq2。
// 给 eq2 补一条刻蚀能力腔体 + 一条交叉资质（eq2 在刻蚀站点 st1 上的资质）。
// 不变量“设备所属站点必须与当前工序一致”要求开工被拒绝。
//
// 交叉资质通过 store 层直接写入：建档服务 CreateQualification 已校验站点一致
// （equipment_service.go），不会接受跨站点资质；此处绕过建档校验直接落库，
// 模拟主数据被错误配置或直接数据操作的场景，用以验证开工校验这最后一道防线。
func TestStartRejectsEquipmentFromOtherStation(t *testing.T) {
	e := newTestEnv(t)
	e.setupAll() // eq1(st1/etch)、eq2(st2/clean)，各站点长窗口资质

	lot := e.registerLot("LOT-XETCH")
	if _, _, err := e.svc.Lot.Enter(e.ctx, lot.ID, ""); err != nil {
		t.Fatalf("进站: %v", err)
	}
	// 批次当前位于刻蚀站点 st1（currentSeq=1）。

	// 给清洗站点的设备 eq2 补一条刻蚀能力腔体（能力标签覆盖 st1 要求的 etch）。
	chEtch, err := e.svc.Master.CreateChamber(e.ctx, e.eq2.ID, "CH-ETCH", "etch")
	if err != nil {
		t.Fatalf("补刻蚀腔体: %v", err)
	}

	// 补一条交叉资质：eq2 在刻蚀站点 st1 上的设备级资质（绕过建档校验直接落库）。
	from, to := baseTime.Add(-time.Hour), baseTime.Add(24*time.Hour)
	crossQual := &domain.Qualification{
		ID:          domain.NewID(domain.IDPrefixQualification),
		EquipmentID: e.eq2.ID,
		StationID:   e.st1.ID,
		ValidFrom:   from,
		ValidTo:     to,
		Status:      domain.QualActive,
		CreatedAt:   baseTime,
	}
	if err := e.store.CreateQualification(e.ctx, crossQual); err != nil {
		t.Fatalf("插交叉资质: %v", err)
	}

	// 开工：设备挂在清洗站点(st2)，批次在刻蚀工序(st1)，站点矛盾必须被拒绝。
	run, _, err := e.svc.Run.CreateRun(e.ctx, lot.ID, e.eq2.ID, chEtch.ID, nil, "")
	if err == nil {
		t.Fatalf("设备所属站点(%s/%s)与当前工序站点(%s/%s)矛盾，开工必须被拒绝，却创建了运行 %s",
			e.eq2.Code, e.st2.Code, e.st1.Code, e.st1.Code, run.ID)
	}
	if !errors.Is(err, domain.ErrCapability) {
		t.Fatalf("站点矛盾应返回能力错误(CAPABILITY)，实际: %v", err)
	}
}
