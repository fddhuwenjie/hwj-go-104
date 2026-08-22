package service_test
import("database/sql";"testing";_ "modernc.org/sqlite";"gowork/wafer/internal/domain")
func TestExpiredRunPaginationKeepsEqualTimestampRows(t *testing.T){e:=newTestEnv(t);e.setupAll();db,err:=sql.Open("sqlite",e.db);if err!=nil{t.Fatal(err)};defer db.Close();for _,code:=range []string{"ER17-A","ER17-B","ER17-C","ER17-D"}{lot:=e.registerLot(code);run:=e.enterAndRun(lot.ID);if _,_,err=e.svc.Run.CompleteRun(e.ctx,run.ID,"");err!=nil{t.Fatal(err)};if _,err=db.Exec(`UPDATE runs SET qual_covered=0 WHERE id=?`,run.ID);err!=nil{t.Fatal(err)}};a,next,err:=e.svc.Query.ExpiredQualificationRuns(e.ctx,domain.Page{Limit:2});if err!=nil{t.Fatal(err)};b,_,err:=e.svc.Query.ExpiredQualificationRuns(e.ctx,domain.Page{Limit:2,Cursor:next});if err!=nil{t.Fatal(err)};seen:=map[string]bool{};for _,x:=range append(a,b...){seen[x.RunID]=true};if len(seen)!=4{t.Fatalf("seen=%d a=%+v b=%+v",len(seen),a,b)}}
func TestAnnotationControlExpiredRunPaginationKeepsEqualTimestampRows(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
