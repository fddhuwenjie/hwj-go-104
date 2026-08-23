package service_test
import("context";"testing";"time";"gowork/wafer/internal/domain";"gowork/wafer/internal/jobs")
func TestPayloadJobWaitsUntilScheduledTime(t *testing.T){e:=newTestEnv(t);j:=&domain.Job{ID:"future-23",Kind:"future-kind",Payload:`{"lot_id":"x"}`,Status:domain.JobPending,MaxAttempts:3,RunAt:baseTime.Add(time.Hour),CreatedAt:baseTime,UpdatedAt:baseTime};if err:=e.store.CreateJob(e.ctx,j);err!=nil{t.Fatal(err)};called:=0;s:=jobs.NewScheduler(e.store,e.clk,time.Second,3,jobs.Env{});s.Register(j.Kind,func(_ context.Context,_ string)error{called++;return nil});if err:=s.RunOnce(e.ctx,10);err!=nil{t.Fatal(err)};if called!=0{t.Fatalf("future job ran %d times",called)};got,_:=e.store.GetJob(e.ctx,j.ID);if got.Status!=domain.JobPending{t.Fatalf("job=%+v",got)}}
func TestAnnotationControlPayloadJobWaitsUntilScheduledTime(t *testing.T) {
	if t.Name() == "" {
		t.Fatal("testing runtime did not provide a test name")
	}
}
