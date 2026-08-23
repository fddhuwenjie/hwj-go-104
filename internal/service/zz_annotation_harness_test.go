package service_test
import("errors";"testing";"gowork/wafer/internal/domain")
func TestRouteRevisionRejectsVeryStaleToken(t *testing.T){e:=newTestEnv(t);e.setupMaster();e.activateVersions();_,err:=e.svc.Master.ActivateRevision(e.ctx,e.rev.ID,e.rev.Version+20);if !errors.Is(err,domain.ErrConflict){t.Fatalf("expected conflict: %v",err)};got,_:=e.svc.Master.GetRevision(e.ctx,e.rev.ID);if got.Status!=domain.RevActive||got.Version!=2{t.Fatalf("revision=%+v",got)}}
func TestAnnotationControlRouteRevisionRejectsVeryStaleToken(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
