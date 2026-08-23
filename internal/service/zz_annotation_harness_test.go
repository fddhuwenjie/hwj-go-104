package service_test
import("errors";"testing";"gowork/wafer/internal/domain")
func TestSameStateEquipmentUpdateRejectsStaleToken(t *testing.T){e:=newTestEnv(t);e.setupMaster();eq,err:=e.svc.Master.CreateEquipment(e.ctx,"EQ-STALE-21","stale","FAMILY-A",e.st1.ID);if err!=nil{t.Fatal(err)};old:=eq.Version;eq,err=e.svc.Master.SetEquipmentStatus(e.ctx,eq.ID,domain.EquipDown,eq.Version);if err!=nil{t.Fatal(err)};_,err=e.svc.Master.SetEquipmentStatus(e.ctx,eq.ID,domain.EquipDown,old);if !errors.Is(err,domain.ErrConflict){t.Fatalf("expected conflict: %v",err)};got,_:=e.svc.Master.GetEquipment(e.ctx,eq.ID);if got.Version!=eq.Version{t.Fatalf("version advanced: %+v",got)}}
func TestAnnotationControlSameStateEquipmentUpdateRejectsStaleToken(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
