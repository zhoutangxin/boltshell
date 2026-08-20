package main

import (
	"fmt"
	"strings"

	"boltshell/internal/db"
)

// ListConnectionGroups 列出连接分组（表 connection_groups）
func (a *App) ListConnectionGroups(includeDeleted bool) ([]db.ConnectionGroup, error) {
	if a.db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	return db.ListGroups(a.db, includeDeleted)
}

// AddConnectionGroup 新增分组
func (a *App) AddConnectionGroup(name string) (db.ConnectionGroup, error) {
	if a.db == nil {
		return db.ConnectionGroup{}, fmt.Errorf("db not initialized")
	}
	return db.InsertGroup(a.db, name)
}

// RenameConnectionGroup 按分组 ID 重命名（连接只存 group_id，名称从分组表读取）
func (a *App) RenameConnectionGroup(groupID, newName string) error {
	if a.db == nil {
		return fmt.Errorf("db not initialized")
	}
	return db.RenameGroup(a.db, groupID, newName)
}

// RenameConnectionGroupByName 按旧名称重命名分组
func (a *App) RenameConnectionGroupByName(oldName, newName string) error {
	if a.db == nil {
		return fmt.Errorf("db not initialized")
	}
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || oldName == "未分组" {
		return fmt.Errorf("该分组不可重命名")
	}
	return db.RenameGroupByName(a.db, oldName, newName)
}

// DeleteConnectionGroupByName 删除分组；组内连接移到未分组。返回被移动的连接数。
func (a *App) DeleteConnectionGroupByName(name string) (int, error) {
	if a.db == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	return db.DeleteGroupByName(a.db, name)
}

// AssignUngroupedToGroup 把「未分组」下的连接归入指定分组
func (a *App) AssignUngroupedToGroup(name string) (int, error) {
	if a.db == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	return db.AssignUngroupedToGroup(a.db, name)
}

// MoveConnection 把连接移到指定分组（空名称表示未分组）
func (a *App) MoveConnection(id, groupName string) error {
	if a.db == nil {
		return fmt.Errorf("db not initialized")
	}
	return db.MoveConnection(a.db, id, groupName)
}
