package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"boltshell/internal/db"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const connectionsExportVersion = 1

// ConnectionsExportBundle SSH 连接导入导出文件格式
type ConnectionsExportBundle struct {
	Version     int                  `json:"version"`
	App         string               `json:"app"`
	ExportedAt  string               `json:"exportedAt"`
	Groups      []db.ConnectionGroup `json:"groups"`
	Connections []db.Connection      `json:"connections"`
}

// ConnectionsImportResult 导入结果摘要
type ConnectionsImportResult struct {
	GroupsAdded      int `json:"GroupsAdded"`
	ConnectionsAdded int `json:"ConnectionsAdded"`
	ConnectionsSkip  int `json:"ConnectionsSkip"`
	ConnectionsUpd   int `json:"ConnectionsUpdated"`
}

// ExportConnections 导出选中的连接（含密码）到用户选择的 JSON 文件
func (a *App) ExportConnections(ids []string) (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("db not initialized")
	}
	if a.ctx == nil {
		return "", fmt.Errorf("runtime not ready")
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("请选择要导出的连接")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出 SSH 连接",
		DefaultFilename: fmt.Sprintf("boltshell-connections-%s.json", time.Now().Format("20060102")),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("已取消导出")
	}
	return a.exportConnectionsToFile(path, ids)
}

func (a *App) exportConnectionsToFile(path string, ids []string) (string, error) {
	want := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		want[id] = struct{}{}
	}
	if len(want) == 0 {
		return "", fmt.Errorf("请选择要导出的连接")
	}

	all, err := db.ListConnections(a.db, true, "")
	if err != nil {
		return "", err
	}
	conns := make([]db.Connection, 0, len(want))
	usedGroups := map[string]struct{}{}
	for _, c := range all {
		if _, ok := want[c.ID]; !ok {
			continue
		}
		conns = append(conns, c)
		gname := strings.TrimSpace(c.GroupName)
		if gname != "" && gname != "未分组" {
			usedGroups[gname] = struct{}{}
		}
	}
	if len(conns) == 0 {
		return "", fmt.Errorf("选中的连接不存在")
	}

	allGroups, err := db.ListGroups(a.db, false)
	if err != nil {
		return "", err
	}
	groups := make([]db.ConnectionGroup, 0, len(usedGroups))
	for _, g := range allGroups {
		if _, ok := usedGroups[g.Name]; ok {
			groups = append(groups, g)
		}
	}

	bundle := ConnectionsExportBundle{
		Version:     connectionsExportVersion,
		App:         "BoltShell",
		ExportedAt:  time.Now().Format(time.RFC3339),
		Groups:      groups,
		Connections: conns,
	}
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// ImportConnections 从 JSON 文件导入分组与连接（同 host+port+user 则更新，否则新增）
func (a *App) ImportConnections() (ConnectionsImportResult, error) {
	var empty ConnectionsImportResult
	if a.db == nil {
		return empty, fmt.Errorf("db not initialized")
	}
	if a.ctx == nil {
		return empty, fmt.Errorf("runtime not ready")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "导入 SSH 连接",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	})
	if err != nil {
		return empty, err
	}
	if strings.TrimSpace(path) == "" {
		return empty, fmt.Errorf("已取消导入")
	}
	return a.importConnectionsFromFile(path)
}

func (a *App) importConnectionsFromFile(path string) (ConnectionsImportResult, error) {
	var result ConnectionsImportResult
	raw, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}

	var bundle ConnectionsExportBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		// 兼容仅连接数组的简单格式
		var only []db.Connection
		if err2 := json.Unmarshal(raw, &only); err2 != nil {
			return result, fmt.Errorf("无法解析导入文件: %v", err)
		}
		bundle.Connections = only
		bundle.Version = connectionsExportVersion
	}
	if bundle.Version > connectionsExportVersion {
		return result, fmt.Errorf("导入文件版本过高（%d），请升级客户端", bundle.Version)
	}

	existingGroups, err := db.ListGroups(a.db, false)
	if err != nil {
		return result, err
	}
	groupByName := map[string]db.ConnectionGroup{}
	for _, g := range existingGroups {
		groupByName[g.Name] = g
	}

	for _, g := range bundle.Groups {
		name := strings.TrimSpace(g.Name)
		if name == "" || name == "未分组" {
			continue
		}
		if _, ok := groupByName[name]; ok {
			continue
		}
		created, err := db.EnsureGroupByName(a.db, name)
		if err != nil {
			return result, err
		}
		groupByName[name] = created
		result.GroupsAdded++
	}

	existing, err := db.ListConnections(a.db, true, "")
	if err != nil {
		return result, err
	}
	type key struct {
		host string
		port int
		user string
	}
	byKey := map[key]db.Connection{}
	for _, c := range existing {
		byKey[key{strings.ToLower(c.Host), c.Port, c.User}] = c
	}

	for _, c := range bundle.Connections {
		host := strings.TrimSpace(c.Host)
		user := strings.TrimSpace(c.User)
		if host == "" || user == "" || c.Password == "" {
			result.ConnectionsSkip++
			continue
		}
		port := c.Port
		if port <= 0 {
			port = 22
		}
		gname := strings.TrimSpace(c.GroupName)
		if gname == "未分组" {
			gname = ""
		}
		if gname != "" {
			if _, ok := groupByName[gname]; !ok {
				created, err := db.EnsureGroupByName(a.db, gname)
				if err != nil {
					return result, err
				}
				groupByName[gname] = created
				result.GroupsAdded++
			}
		}

		k := key{strings.ToLower(host), port, user}
		if old, ok := byKey[k]; ok {
			upd := old
			upd.Name = c.Name
			upd.Host = host
			upd.Port = port
			upd.User = user
			upd.Password = c.Password
			upd.GroupName = gname
			if c.Enabled == 0 || c.Enabled == 1 {
				upd.Enabled = c.Enabled
			}
			if err := db.UpdateConnection(a.db, upd); err != nil {
				return result, err
			}
			result.ConnectionsUpd++
			continue
		}

		id := db.NewID()
		en := c.Enabled
		if en != 0 && en != 1 {
			en = 1
		}
		if err := db.InsertConnection(a.db, db.Connection{
			ID:        id,
			Name:      c.Name,
			Host:      host,
			Port:      port,
			User:      user,
			Password:  c.Password,
			GroupName: gname,
			Enabled:   en,
			Deleted:   0,
			CreatedAt: time.Now().Unix(),
		}); err != nil {
			return result, err
		}
		result.ConnectionsAdded++
	}
	return result, nil
}
