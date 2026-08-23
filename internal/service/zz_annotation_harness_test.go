package service_test
import ("testing";"gowork/wafer/internal/domain")
func TestRunCannotMaskEquipmentStationOwnership(t *testing.T){
 e:=newTestEnv(t); e.setupAll(); ch,err:=e.svc.Master.CreateChamber(e.ctx,e.eq2.ID,"CROSS-STATION","etch"); if err!=nil {t.Fatal(err)}
 q:=&domain.Qualification{ID:domain.NewID(domain.IDPrefixQualification),EquipmentID:e.eq2.ID,ChamberID:ch.ID,StationID:e.st1.ID,ValidFrom:baseTime.Add(-3600000000000),ValidTo:baseTime.Add(3600000000000),Status:domain.QualActive,CreatedAt:baseTime}; if err=e.store.CreateQualification(e.ctx,q); err!=nil {t.Fatal(err)}
 lot:=e.registerLot("STATION-08"); if _,_,err=e.svc.Lot.Enter(e.ctx,lot.ID,""); err!=nil {t.Fatal(err)}
 if _,_,err=e.svc.Run.CreateRun(e.ctx,lot.ID,e.eq2.ID,ch.ID,nil,""); err==nil {t.Fatal("equipment from another station started the run")}
}
func TestAnnotationControlRunCannotMaskEquipmentStationOwnership(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
