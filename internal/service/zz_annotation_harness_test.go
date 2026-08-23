package service_test
import ("errors";"testing";"gowork/wafer/internal/domain")
func TestPlanActivationRejectsStaleRevisionToken(t *testing.T){
 e:=newTestEnv(t); p,err:=e.svc.Master.CreatePlan(e.ctx,"STALE-03","plan","cd",[]int{1},1,5); if err!=nil {t.Fatal(err)}
 _,err=e.svc.Master.ActivatePlan(e.ctx,p.ID,p.RowVersion+9)
 if !errors.Is(err,domain.ErrConflict){t.Fatalf("expected conflict, got %v",err)}
 got,_:=e.svc.Master.GetPlan(e.ctx,p.ID); if got.Status!=domain.PlanDraft || got.RowVersion!=p.RowVersion {t.Fatalf("plan changed: %+v",got)}
}
func TestAnnotationControlPlanActivationRejectsStaleRevisionToken(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
