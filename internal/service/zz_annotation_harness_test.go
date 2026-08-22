package service_test
import "testing"
func TestRunCannotMaskForeignChamberOwnership(t *testing.T){
 e:=newTestEnv(t); e.setupAll(); foreign,err:=e.svc.Master.CreateChamber(e.ctx,e.eq2.ID,"FOREIGN-ETCH","etch"); if err!=nil {t.Fatal(err)}
 lot:=e.registerLot("FOREIGN-02"); if _,_,err=e.svc.Lot.Enter(e.ctx,lot.ID,""); err!=nil {t.Fatal(err)}
 if _,_,err=e.svc.Run.CreateRun(e.ctx,lot.ID,e.eq1.ID,foreign.ID,nil,""); err==nil {t.Fatal("run accepted another equipment's chamber")}
}
func TestAnnotationControlRunCannotMaskForeignChamberOwnership(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
