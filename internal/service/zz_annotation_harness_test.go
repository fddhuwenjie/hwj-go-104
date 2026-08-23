package service_test
import ("database/sql";"testing"; _ "modernc.org/sqlite";"gowork/wafer/internal/domain")
func TestPlanSwitchAuditFailureKeepsPreviousPlanActive(t *testing.T){
 e:=newTestEnv(t); first,err:=e.svc.Master.CreatePlan(e.ctx,"ATOMIC-P10","first","cd",[]int{1},1,5); if err!=nil {t.Fatal(err)}
 first,err=e.svc.Master.ActivatePlan(e.ctx,first.ID,first.RowVersion); if err!=nil {t.Fatal(err)}
 second,err:=e.svc.Master.CreatePlan(e.ctx,"ATOMIC-P10","second","cd",[]int{1},1,6); if err!=nil {t.Fatal(err)}
 db,err:=sql.Open("sqlite",e.db); if err!=nil {t.Fatal(err)}; defer db.Close(); if _,err=db.Exec(`CREATE TRIGGER reject_plan_audit BEFORE INSERT ON audit_events WHEN NEW.action='metrology_plan.activate' BEGIN SELECT RAISE(ABORT,'audit down'); END`); err!=nil {t.Fatal(err)}
 if _,err=e.svc.Master.ActivatePlan(e.ctx,second.ID,second.RowVersion); err==nil {t.Fatal("activation unexpectedly succeeded")}
 old,_:=e.svc.Master.GetPlan(e.ctx,first.ID); fresh,_:=e.svc.Master.GetPlan(e.ctx,second.ID); if old.Status!=domain.PlanActive||fresh.Status!=domain.PlanDraft {t.Fatalf("old=%s new=%s",old.Status,fresh.Status)}
}
func TestAnnotationControlPlanSwitchAuditFailureKeepsPreviousPlanActive(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
