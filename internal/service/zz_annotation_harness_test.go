package service_test
import("testing";"gowork/wafer/internal/domain")
func TestWipViewKeepsEachLotsLatestHoldReason(t *testing.T){
 e:=newTestEnv(t);e.setupAll();a:=e.registerLot("WIP-A12");b:=e.registerLot("WIP-B12")
 if _,_,err:=e.svc.Hold.CreateHold(e.ctx,a.ID,"first-a","");err!=nil{t.Fatal(err)};if _,_,err:=e.svc.Hold.CreateHold(e.ctx,a.ID,"latest-a","");err!=nil{t.Fatal(err)}
 if _,_,err:=e.svc.Hold.CreateHold(e.ctx,b.ID,"only-b","");err!=nil{t.Fatal(err)}
 rows,_,err:=e.svc.Query.WipLots(e.ctx,domain.Page{Limit:10});if err!=nil{t.Fatal(err)}; got:=map[string]string{};for _,r:=range rows{got[r.LotID]=r.LatestHoldReason}
 if got[a.ID]!="latest-a"||got[b.ID]!="only-b"{t.Fatalf("reasons=%v",got)}
}
func TestAnnotationControlWipViewKeepsEachLotsLatestHoldReason(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
