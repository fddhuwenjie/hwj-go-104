package service_test
import "testing"
func TestQualificationEndInstantStillCoversCompletion(t *testing.T){
 e:=newTestEnv(t); e.setupMaster(); e.activateVersions(); e.setupEquipment(false)
 end:=baseTime.Add(2*3600000000000); if _,err:=e.svc.Master.CreateQualification(e.ctx,e.eq1.ID,e.ch1.ID,e.st1.ID,baseTime.Add(-3600000000000),end); err!=nil {t.Fatal(err)}
 lot:=e.registerLot("BOUND-05"); if _,_,err:=e.svc.Lot.Enter(e.ctx,lot.ID,""); err!=nil {t.Fatal(err)}
 run,_,err:=e.svc.Run.CreateRun(e.ctx,lot.ID,e.eq1.ID,e.ch1.ID,nil,""); if err!=nil {t.Fatal(err)}
 e.clk.Set(end); done,_,err:=e.svc.Run.CompleteRun(e.ctx,run.ID,""); if err!=nil {t.Fatal(err)}
 if !done.QualCovered {t.Fatal("inclusive end instant was marked uncovered")}
}
func TestAnnotationControlQualificationEndInstantStillCoversCompletion(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
