package main
import (
  "database/sql"
  "fmt"
  _ "modernc.org/sqlite"
)
func main(){
  db, err := sql.Open("sqlite", `D:/dev/code/mobilerpa/repos/mobilerpa-center/server/data/mobilerpa.db`)
  if err != nil { panic(err) }
  defer db.Close()
  row := db.QueryRow(`SELECT t.id, t.workflow_run_id, t.workflow_node_id, COALESCE(r.workflow_def_id, 0), t.status, t.result_message FROM tasks t LEFT JOIN workflow_runs r ON r.id = t.workflow_run_id WHERE t.id = ?`, 1)
  var a,b,c,d,e,f string
  err = row.Scan(&a,&b,&c,&d,&e,&f)
  fmt.Printf("scan err=%v values=%q|%q|%q|%q|%q|%q\n", err,a,b,c,d,e,f)
}
