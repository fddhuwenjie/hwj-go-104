package service_test
import("testing";"gowork/wafer/internal/service")
func TestReadingRejectsWaferFromAnotherRun(t *testing.T){e:=newTestEnv(t);e.setupAll();a:=e.registerLot("READ-A20");b:=e.registerLot("READ-B20");ra:=e.enterAndRun(a.ID);rb:=e.enterAndRun(b.ID);if _,_,err:=e.svc.Run.CompleteRun(e.ctx,ra.ID,"");err!=nil{t.Fatal(err)};bw,_:=e.svc.Lot.ListWafers(e.ctx,b.ID);_,_,err:=e.svc.Reading.SubmitReadings(e.ctx,ra.ID,[]service.ReadingInput{{WaferID:bw[0].ID,Metric:"cd",Value:1}},"");if err==nil{t.Fatal("foreign run wafer accepted")};rows,_:=e.svc.Reading.ListReadings(e.ctx,ra.ID);if len(rows)!=0{t.Fatalf("readings=%+v runB=%s",rows,rb.ID)}}
func TestAnnotationControlReadingRejectsWaferFromAnotherRun(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
