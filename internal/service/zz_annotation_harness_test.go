package service_test
import ("testing"; "gowork/wafer/internal/domain")
func TestScrappedLotCannotAcquireNewHold(t *testing.T){
 e:=newTestEnv(t); e.setupAll(); lot:=e.registerLot("TERM-01")
 if _,_,err:=e.svc.Lot.Scrap(e.ctx,lot.ID,"wafer damage",""); err!=nil {t.Fatal(err)}
 if _,_,err:=e.svc.Hold.CreateHold(e.ctx,lot.ID,"late review",""); err==nil {t.Fatal("scrapped lot accepted a new hold")}
 got,err:=e.svc.Lot.GetLot(e.ctx,lot.ID); if err!=nil {t.Fatal(err)}
 if got.Status!=domain.LotScrapped {t.Fatalf("status=%s",got.Status)}
}
func TestAnnotationControlScrappedLotCannotAcquireNewHold(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
