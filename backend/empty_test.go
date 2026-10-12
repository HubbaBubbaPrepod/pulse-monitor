package main

import (
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
)

func TestCleanRegistryAndLastNodeDeletion(t *testing.T) {
 dir := t.TempDir()
 a, err := newApp(dir)
 if err != nil { t.Fatal(err) }
 if len(a.serverList()) != 0 { t.Fatal("clean installation must not seed servers") }
 if a.dataSource() != "waiting" { t.Fatal("empty registry must wait for agents") }
 body := `{"id":"node","name":"node","address":"https://node.example","os":"","group":"Test","online":false,"cpu":0,"memory":0,"disk":0,"interval":5,"threshold":85}`
 if w:=request(t,a,"PUT","/api/servers/node",body); w.Code!=200 { t.Fatal(w.Code,w.Body.String()) }
 if w:=request(t,a,"DELETE","/api/servers/node",""); w.Code!=200 { t.Fatal(w.Code,w.Body.String()) }
 a,err=newApp(dir); if err!=nil {t.Fatal(err)}
 if len(a.serverList())!=0 {t.Fatal("empty registry must survive restart")}
}

func TestDemoMigrationPreservesRealRecords(t *testing.T) {
 dir:=t.TempDir()
 nodes:=[]server{{ID:"demo-node",Name:"Demo",Source:"demo",Address:"https://demo.example",Interval:5,Threshold:85},{ID:"real",Name:"Real",Source:"waiting",Address:"https://real.example",Interval:5,Threshold:85}}
 raw,_:=json.Marshal(nodes); if err:=os.WriteFile(filepath.Join(dir,"servers.json"),raw,0600);err!=nil {t.Fatal(err)}
 a,err:=newApp(dir);if err!=nil {t.Fatal(err)}
 if got:=a.serverList();len(got)!=1||got[0].ID!="real" {t.Fatalf("migration retained wrong nodes: %+v",got)}
 if _,err:=os.Stat(filepath.Join(dir,"servers.pre-real.json"));err!=nil {t.Fatal("migration must retain backup",err)}
}
