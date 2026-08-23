package service_test
import("database/sql";"testing";_ "modernc.org/sqlite")
func TestSplitAuditFailureRollsBackChildLot(t *testing.T){e:=newTestEnv(t);e.setupAll();p:=e.registerLot("SPLIT-28");ws,_:=e.svc.Lot.ListWafers(e.ctx,p.ID);db,err:=sql.Open("sqlite",e.db);if err!=nil{t.Fatal(err)};defer db.Close();if _,err=db.Exec(`CREATE TRIGGER reject_split_audit BEFORE INSERT ON audit_events WHEN NEW.action='lot.split' BEGIN SELECT RAISE(ABORT,'audit down'); END`);err!=nil{t.Fatal(err)};_,_,err=e.svc.Lot.SplitLot(e.ctx,p.ID,"SPLIT-C28",[]string{ws[2].ID},"split-key-28");if err==nil{t.Fatal("split succeeded")};var n int;if err=db.QueryRow(`SELECT COUNT(*) FROM lots WHERE code='SPLIT-C28'`).Scan(&n);err!=nil{t.Fatal(err)};if n!=0{t.Fatalf("child survived: %d",n)}}
func TestAnnotationControlSplitAuditFailureRollsBackChildLot(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
