package service_test
import("errors";"testing";"gowork/wafer/internal/domain")
func TestActivePlanRejectsStaleReactivation(t *testing.T){e:=newTestEnv(t);p,err:=e.svc.Master.CreatePlan(e.ctx,"PLAN-27","p","cd",[]int{1},1,8);if err!=nil{t.Fatal(err)};old:=p.RowVersion;p,err=e.svc.Master.ActivatePlan(e.ctx,p.ID,p.RowVersion);if err!=nil{t.Fatal(err)};_,err=e.svc.Master.ActivatePlan(e.ctx,p.ID,old);if !errors.Is(err,domain.ErrConflict){t.Fatalf("expected conflict: %v",err)};got,_:=e.svc.Master.GetPlan(e.ctx,p.ID);if got.RowVersion!=p.RowVersion{t.Fatalf("plan=%+v",got)}}
func TestAnnotationControlActivePlanRejectsStaleReactivation(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
