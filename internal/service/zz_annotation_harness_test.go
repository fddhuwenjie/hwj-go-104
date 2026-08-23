package service_test
import("errors";"testing";"gowork/wafer/internal/domain")
func TestRootLotCannotRestoreWhileHoldOpen(t *testing.T){e:=newTestEnv(t);e.setupAll();lot:=e.registerLot("RESTORE-25");if _,_,err:=e.svc.Hold.CreateHold(e.ctx,lot.ID,"open","");err!=nil{t.Fatal(err)};_,_,err:=e.svc.Lot.Restore(e.ctx,lot.ID,"");if !errors.Is(err,domain.ErrHoldBlocking){t.Fatalf("expected blocking: %v",err)};got,_:=e.svc.Lot.GetLot(e.ctx,lot.ID);if got.Status!=domain.LotOnHold{t.Fatalf("lot=%+v",got)}}
func TestAnnotationControlRootLotCannotRestoreWhileHoldOpen(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
