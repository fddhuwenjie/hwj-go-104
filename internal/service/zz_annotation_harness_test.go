package service_test
import "testing"
func TestSplitRejectsWaferOwnedBySiblingLot(t *testing.T){
 e:=newTestEnv(t);e.setupAll();a:=e.registerLot("SPLIT-A14");b:=e.registerLot("SPLIT-B14");bw,_:=e.svc.Lot.ListWafers(e.ctx,b.ID)
 if _,_,err:=e.svc.Lot.SplitLot(e.ctx,a.ID,"SPLIT-C14",[]string{bw[0].ID},"");err==nil{t.Fatal("foreign wafer moved")}
 got,_:=e.svc.Lot.ListWafers(e.ctx,b.ID);if len(got)!=3{t.Fatalf("source wafers=%d",len(got))}
}
func TestAnnotationControlSplitRejectsWaferOwnedBySiblingLot(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
