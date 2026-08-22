package service_test
import ("errors";"testing";"gowork/wafer/internal/domain")
func TestGrandchildHoldBlocksAncestorEntry(t *testing.T){
 e:=newTestEnv(t); e.setupAll(); p:=e.registerLot("TREE-07"); ws,_:=e.svc.Lot.ListWafers(e.ctx,p.ID)
 child,_,err:=e.svc.Lot.SplitLot(e.ctx,p.ID,"TREE-07-C",[]string{ws[1].ID,ws[2].ID},""); if err!=nil {t.Fatal(err)}
 cws,_:=e.svc.Lot.ListWafers(e.ctx,child.ID); grand,_,err:=e.svc.Lot.SplitLot(e.ctx,child.ID,"TREE-07-G",[]string{cws[1].ID},""); if err!=nil {t.Fatal(err)}
 if _,_,err=e.svc.Hold.CreateHold(e.ctx,grand.ID,"deep anomaly",""); err!=nil {t.Fatal(err)}
 _,_,err=e.svc.Lot.Enter(e.ctx,p.ID,""); if !errors.Is(err,domain.ErrHoldBlocking){t.Fatalf("ancestor entry error=%v",err)}
}
func TestAnnotationControlGrandchildHoldBlocksAncestorEntry(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
