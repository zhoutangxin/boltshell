package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := InitSchema(d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestInitSchemaTwoTables(t *testing.T) {
	d := openTestDB(t)
	for _, name := range []string{"connections", "connection_groups"} {
		exists, err := tableExists(d, name)
		if err != nil || !exists {
			t.Fatalf("%s table: exists=%v err=%v", name, exists, err)
		}
	}
	if exists, err := columnExists(d, "connections", "group_name"); err != nil || exists {
		t.Fatalf("connections.group_name should not exist: exists=%v err=%v", exists, err)
	}
}

func TestGroupAndConnectionSeparateTables(t *testing.T) {
	d := openTestDB(t)
	g, err := InsertGroup(d, "aliyun")
	if err != nil {
		t.Fatal(err)
	}
	if g.Status != GroupStatusActive {
		t.Fatalf("status=%s", g.Status)
	}
	if err := InsertConnection(d, Connection{
		ID: "c1", Name: "web", Host: "1.1.1.1", Port: 22, User: "root", Password: "p",
		GroupName: "aliyun", Enabled: 1, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	conns, err := ListConnections(d, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].GroupName != "aliyun" || conns[0].GroupID != g.ID {
		t.Fatalf("conns=%+v", conns)
	}
	groups, err := ListGroups(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Name != "aliyun" {
		t.Fatalf("groups=%+v", groups)
	}
	if _, err := GetByID(d, g.ID); err == nil {
		t.Fatal("GetByID on group id should not find a connection")
	}
}

func TestRenameGroupUpdatesConnections(t *testing.T) {
	d := openTestDB(t)
	g, err := InsertGroup(d, "old")
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertConnection(d, Connection{
		ID: "c1", Host: "h", Port: 22, User: "root", Password: "p",
		GroupName: "old", Enabled: 1, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := RenameGroupByName(d, "old", "new"); err != nil {
		t.Fatal(err)
	}
	conns, _ := ListConnections(d, false, "")
	if conns[0].GroupName != "new" || conns[0].GroupID != g.ID {
		t.Fatalf("conn=%+v", conns[0])
	}
	groups, _ := ListGroups(d, false)
	if len(groups) != 1 || groups[0].Name != "new" {
		t.Fatalf("groups=%+v", groups)
	}
}

func TestDeleteGroupMovesConnections(t *testing.T) {
	d := openTestDB(t)
	if err := InsertConnection(d, Connection{
		ID: "c1", Host: "h", Port: 22, User: "root", Password: "p",
		GroupName: "g1", Enabled: 1, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	moved, err := DeleteGroupByName(d, "g1")
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved=%d", moved)
	}
	conns, _ := ListConnections(d, false, "")
	if conns[0].GroupName != "" || conns[0].GroupID != "" {
		t.Fatalf("expected ungrouped, got %+v", conns[0])
	}
	groups, _ := ListGroups(d, false)
	if len(groups) != 0 {
		t.Fatalf("groups=%+v", groups)
	}
}

func TestInsertGroupRejectsDuplicate(t *testing.T) {
	d := openTestDB(t)
	if _, err := InsertGroup(d, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := InsertGroup(d, "a"); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestMigrateFromSingleTable(t *testing.T) {
	d, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "single.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	_, err = d.Exec(`
CREATE TABLE connections (
  id TEXT PRIMARY KEY,
  name TEXT,
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  user TEXT NOT NULL DEFAULT '',
  password TEXT NOT NULL DEFAULT '',
  group_name TEXT,
  item_type TEXT NOT NULL DEFAULT 'connection',
  status TEXT NOT NULL DEFAULT 'active',
  enabled INTEGER NOT NULL DEFAULT 1,
  deleted INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
INSERT INTO connections(id,name,host,port,user,password,group_name,item_type,status,enabled,deleted,created_at)
 VALUES('g1','aliyun','',0,'','','aliyun','group','active',0,0,1);
INSERT INTO connections(id,name,host,port,user,password,group_name,item_type,status,enabled,deleted,created_at)
 VALUES('c1','web','1.2.3.4',22,'root','p','aliyun','connection','active',1,0,1);
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(d); err != nil {
		t.Fatal(err)
	}
	exists, _ := tableExists(d, "connection_groups")
	if !exists {
		t.Fatal("connection_groups should exist")
	}
	groups, err := ListGroups(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Name != "aliyun" {
		t.Fatalf("groups=%+v", groups)
	}
	conns, err := ListConnections(d, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].Host != "1.2.3.4" || conns[0].GroupName != "aliyun" || conns[0].GroupID == "" {
		t.Fatalf("conns=%+v", conns)
	}
	if _, err := GetByID(d, "g1"); err == nil {
		t.Fatal("embedded group row should have been removed from connections")
	}
	if exists, _ := columnExists(d, "connections", "group_name"); exists {
		t.Fatal("legacy group_name column should be dropped")
	}
}

func TestMigrateLegacyGroupNameOnly(t *testing.T) {
	d, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	_, err = d.Exec(`
CREATE TABLE connections (
  id TEXT PRIMARY KEY,
  name TEXT,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  user TEXT NOT NULL,
  password TEXT NOT NULL,
  group_name TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  deleted INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
INSERT INTO connections(id,name,host,port,user,password,group_name,enabled,deleted,created_at)
 VALUES('c1','web','1.2.3.4',22,'root','p','aliyun',1,0,1);
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(d); err != nil {
		t.Fatal(err)
	}
	groups, err := ListGroups(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Name != "aliyun" {
		t.Fatalf("groups=%+v", groups)
	}
	conns, err := ListConnections(d, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(conns) != 1 || conns[0].GroupID != groups[0].ID || conns[0].GroupName != "aliyun" {
		t.Fatalf("conns=%+v groups=%+v", conns, groups)
	}
	if exists, _ := columnExists(d, "connections", "group_name"); exists {
		t.Fatal("legacy group_name column should be dropped")
	}
}

func TestAssignUngroupedToGroup(t *testing.T) {
	d := openTestDB(t)
	if err := InsertConnection(d, Connection{
		ID: "c1", Host: "h", Port: 22, User: "root", Password: "p",
		Enabled: 1, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	moved, err := AssignUngroupedToGroup(d, "aliyun")
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved=%d", moved)
	}
	conns, _ := ListConnections(d, false, "")
	if conns[0].GroupName != "aliyun" || conns[0].GroupID == "" {
		t.Fatalf("conn=%+v", conns[0])
	}
}

func TestMoveConnection(t *testing.T) {
	d := openTestDB(t)
	g, err := InsertGroup(d, "aliyun")
	if err != nil {
		t.Fatal(err)
	}
	if err := InsertConnection(d, Connection{
		ID: "c1", Host: "h", Port: 22, User: "root", Password: "p",
		Enabled: 1, CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := MoveConnection(d, "c1", "aliyun"); err != nil {
		t.Fatal(err)
	}
	c, err := GetByID(d, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if c.GroupID != g.ID || c.GroupName != "aliyun" {
		t.Fatalf("moved=%+v", c)
	}
	if err := MoveConnection(d, "c1", ""); err != nil {
		t.Fatal(err)
	}
	c, _ = GetByID(d, "c1")
	if c.GroupID != "" || c.GroupName != "" {
		t.Fatalf("ungrouped=%+v", c)
	}
}
