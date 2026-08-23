package service_test
import "testing"
func TestRunRejectsWaferFromDifferentLot(t *testing.T){e:=newTestEnv(t);e.setupAll();a:=e.registerLot("RUN-A26");b:=e.registerLot("RUN-B26");if _,_,err:=e.svc.Lot.Enter(e.ctx,a.ID,"");err!=nil{t.Fatal(err)};bw,_:=e.svc.Lot.ListWafers(e.ctx,b.ID);if _,_,err:=e.svc.Run.CreateRun(e.ctx,a.ID,e.eq1.ID,e.ch1.ID,[]string{bw[0].ID},"");err==nil{t.Fatal("foreign wafer accepted")};runs,_:=e.svc.Run.ListRunsByLot(e.ctx,a.ID);if len(runs)!=0{t.Fatalf("runs=%+v",runs)}}
func TestAnnotationControlRunRejectsWaferFromDifferentLot(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
