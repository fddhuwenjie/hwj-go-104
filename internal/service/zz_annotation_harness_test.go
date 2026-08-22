package service_test
import ("encoding/json";"errors";"testing";"gowork/wafer/internal/domain")
func TestRecipeActivationRejectsObsoleteRowVersion(t *testing.T){
 e:=newTestEnv(t); r,err:=e.svc.Master.CreateRecipe(e.ctx,"STALE-R9","recipe","FAMILY-A"); if err!=nil {t.Fatal(err)}
 v,err:=e.svc.Master.CreateRecipeVersion(e.ctx,r.ID,json.RawMessage(`{"temp":77}`)); if err!=nil {t.Fatal(err)}
 _,err=e.svc.Master.ActivateRecipeVersion(e.ctx,v.ID,v.RowVersion+6); if !errors.Is(err,domain.ErrConflict){t.Fatalf("expected conflict, got %v",err)}
 got,_:=e.svc.Master.GetRecipeVersion(e.ctx,v.ID); if got.Status!=domain.RecipeDraft || got.Snapshot!="" {t.Fatalf("version changed: %+v",got)}
}
func TestAnnotationControlRecipeActivationRejectsObsoleteRowVersion(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
