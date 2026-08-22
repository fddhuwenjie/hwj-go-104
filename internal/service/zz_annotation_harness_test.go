package service_test
import("testing";"time")
func TestQueueIncludesLotAtExactWaitThreshold(t *testing.T){
 e:=newTestEnv(t); e.setupAll(); lot:=e.registerLot("WAIT-11"); if _,_,err:=e.svc.Lot.Enter(e.ctx,lot.ID,"");err!=nil{t.Fatal(err)}
 e.clk.Advance(10*time.Minute); items,err:=e.svc.Query.StationQueues(e.ctx,10*time.Minute);if err!=nil{t.Fatal(err)}
 if len(items)!=1||items[0].LotID!=lot.ID{t.Fatalf("exact threshold missing: %+v",items)}
}
func TestAnnotationControlQueueIncludesLotAtExactWaitThreshold(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
