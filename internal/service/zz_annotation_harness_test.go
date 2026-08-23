package service_test
import("encoding/json";"testing";"gowork/wafer/internal/domain")
func TestWipViewUsesEachLotsFrozenRevision(t *testing.T){e:=newTestEnv(t);e.setupAll();old:=e.registerLot("WIP-OLD30");if _,_,err:=e.svc.Lot.Enter(e.ctx,old.ID,"");err!=nil{t.Fatal(err)};v,err:=e.svc.Master.CreateRecipeVersion(e.ctx,e.rc1.ID,json.RawMessage(`{"temp":200}`));if err!=nil{t.Fatal(err)};if _,err=e.svc.Master.ActivateRecipeVersion(e.ctx,v.ID,v.RowVersion);err!=nil{t.Fatal(err)};rev2,err:=e.svc.Master.CreateRevision(e.ctx,e.route.ID,[]domain.RouteStation{{Seq:1,StationID:e.st1.ID,RecipeID:e.rc1.ID,MetrologyPlanID:e.plan1.ID},{Seq:2,StationID:e.st2.ID,RecipeID:e.rc2.ID,MetrologyPlanID:e.plan2.ID}});if err!=nil{t.Fatal(err)};if _,err=e.svc.Master.ActivateRevision(e.ctx,rev2.ID,rev2.Version);err!=nil{t.Fatal(err)};fresh:=e.registerLot("WIP-NEW30");if _,_,err=e.svc.Lot.Enter(e.ctx,fresh.ID,"");err!=nil{t.Fatal(err)};rows,_,err:=e.svc.Query.WipLots(e.ctx,domain.Page{Limit:10});if err!=nil{t.Fatal(err)};got:=map[string]int{};for _,x:=range rows{if x.FrozenRevision!=nil{got[x.LotID]=*x.FrozenRevision}};if got[old.ID]!=1||got[fresh.ID]!=2{t.Fatalf("revisions=%v",got)}}
func TestAnnotationControlWipViewUsesEachLotsFrozenRevision(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
