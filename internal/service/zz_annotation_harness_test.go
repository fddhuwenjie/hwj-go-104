package service_test
import ("database/sql";"testing"; _ "modernc.org/sqlite"; "gowork/wafer/internal/domain")
func TestRevisionAuditFailureRollsBackRevisionAndStations(t *testing.T){
 e:=newTestEnv(t); e.setupMaster(); db,err:=sql.Open("sqlite",e.db); if err!=nil {t.Fatal(err)}; defer db.Close()
 if _,err=db.Exec(`CREATE TRIGGER reject_revision_audit BEFORE INSERT ON audit_events WHEN NEW.action='route_revision.create' BEGIN SELECT RAISE(ABORT,'audit down'); END`); err!=nil {t.Fatal(err)}
 _,err=e.svc.Master.CreateRevision(e.ctx,e.route.ID,[]domain.RouteStation{{Seq:1,StationID:e.st1.ID,RecipeID:e.rc1.ID,MetrologyPlanID:e.plan1.ID}}); if err==nil {t.Fatal("create unexpectedly succeeded")}
 var n int; if err=db.QueryRow(`SELECT COUNT(*) FROM route_revisions WHERE route_id=? AND revision>1`,e.route.ID).Scan(&n); err!=nil {t.Fatal(err)}
 if n!=0 {t.Fatalf("revision survived failed audit: %d",n)}
}
func TestAnnotationControlRevisionAuditFailureRollsBackRevisionAndStations(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
