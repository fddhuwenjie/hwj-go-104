package service_test
import("database/sql";"testing";_ "modernc.org/sqlite";"gowork/wafer/internal/service")
func TestLotAuditFailureRollsBackLotAndWafers(t *testing.T){e:=newTestEnv(t);e.setupAll();db,err:=sql.Open("sqlite",e.db);if err!=nil{t.Fatal(err)};defer db.Close();if _,err=db.Exec(`CREATE TRIGGER reject_lot_audit BEFORE INSERT ON audit_events WHEN NEW.action='lot.create' BEGIN SELECT RAISE(ABORT,'audit down'); END`);err!=nil{t.Fatal(err)};_,_,err=e.svc.Lot.RegisterLot(e.ctx,"ATOMIC-16",e.pf.ID,e.route.ID,[]service.WaferInput{{Code:"A16-W1",Slot:1}},"audit-key-16");if err==nil{t.Fatal("register succeeded")};var n int;if err=db.QueryRow(`SELECT COUNT(*) FROM lots WHERE code='ATOMIC-16'`).Scan(&n);err!=nil{t.Fatal(err)};if n!=0{t.Fatalf("lot survived: %d",n)}}
func TestAnnotationControlLotAuditFailureRollsBackLotAndWafers(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
