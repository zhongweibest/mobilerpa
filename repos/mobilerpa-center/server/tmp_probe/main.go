package main
import (
  "context"
  "fmt"
  "github.com/mobilerpa/mobilerpa-center/server/internal/device"
  "github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
  "github.com/mobilerpa/mobilerpa-center/server/internal/storage"
  "github.com/mobilerpa/mobilerpa-center/server/internal/task"
  "github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
)
func main(){
  db, err := storage.Open(`D:/dev/code/mobilerpa/repos/mobilerpa-center/server/data/mobilerpa.db`)
  if err != nil { panic(err) }
  defer db.Close()
  ds := device.NewService(db)
  ts := task.NewService(db)
  disp := dispatch.NewService(ts)
  ws := workflow.NewService(db, ds, ts, disp)
  err = ws.HandleTaskResult(context.Background(), "1")
  if err != nil { fmt.Printf("ERR: %v\n", err); return }
  fmt.Println("OK")
}
