package service_test
import("testing";"time";"gowork/wafer/internal/domain";"gowork/wafer/internal/jobs")
func TestReleasedHoldIsNeverEscalatedAgain(t *testing.T){e:=newTestEnv(t);e.setupAll();lot:=e.registerLot("HOLD-19");hold,_,err:=e.svc.Hold.CreateHold(e.ctx,lot.ID,"manual","");if err!=nil{t.Fatal(err)};if _,_,err=e.svc.Hold.Review(e.ctx,hold.ID,domain.ReviewRelease,"ok",0,"");err!=nil{t.Fatal(err)};e.clk.Advance(2*time.Hour);fn:=jobs.HoldEscalationHandler(e.store,e.clk,time.Hour);if err=fn(e.ctx,"");err!=nil{t.Fatal(err)};got,err:=e.svc.Hold.GetHold(e.ctx,hold.ID);if err!=nil{t.Fatal(err)};if got.Escalated||got.Status!=domain.HoldReleased{t.Fatalf("hold=%+v",got)}}
func TestAnnotationControlReleasedHoldIsNeverEscalatedAgain(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
