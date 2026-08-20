package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"boltshell/internal/db"
)

func TestImportConnectionsFromFile(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.InitSchema(d); err != nil {
		t.Fatal(err)
	}
	a := &App{db: d}

	bundle := ConnectionsExportBundle{
		Version: connectionsExportVersion,
		App:     "BoltShell",
		Groups: []db.ConnectionGroup{
			{Name: "aliyun"},
		},
		Connections: []db.Connection{
			{Name: "web", Host: "1.1.1.1", Port: 22, User: "root", Password: "secret", GroupName: "aliyun", Enabled: 1},
		},
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "in.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := a.importConnectionsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.GroupsAdded != 1 || res.ConnectionsAdded != 1 {
		t.Fatalf("result=%+v", res)
	}

	res2, err := a.importConnectionsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if res2.ConnectionsAdded != 0 || res2.ConnectionsUpd != 1 {
		t.Fatalf("second import=%+v", res2)
	}

	groups, _ := db.ListGroups(d, false)
	if len(groups) != 1 || groups[0].Name != "aliyun" {
		t.Fatalf("groups=%+v", groups)
	}
}

func TestExportConnectionsToFileSelected(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.InitSchema(d); err != nil {
		t.Fatal(err)
	}
	a := &App{db: d}
	id1, err := a.AddConnection("a", "1.1.1.1", 22, "root", "p", "g1", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddConnection("b", "2.2.2.2", 22, "root", "p", "g2", true); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.json")
	if _, err := a.exportConnectionsToFile(out, []string{id1}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var bundle ConnectionsExportBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.Connections) != 1 || bundle.Connections[0].Host != "1.1.1.1" {
		t.Fatalf("exported=%+v", bundle.Connections)
	}
	if len(bundle.Groups) != 1 || bundle.Groups[0].Name != "g1" {
		t.Fatalf("groups=%+v", bundle.Groups)
	}
}
