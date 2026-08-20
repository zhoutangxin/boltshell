package db // 数据库访问层：封装 SQLite 相关操作

import (
	"crypto/rand"          // 标准库：安全随机数，用于生成 ID
	"database/sql"         // 标准库：通用数据库接口
	"fmt"                  // 标准库：错误信息
	_ "modernc.org/sqlite" // 第三方：纯 Go 的 SQLite 驱动（仅导入以注册驱动）
	"os"                   // 标准库：可执行文件路径获取
	"path/filepath"        // 标准库：路径拼接
	"strings"
	"time"
)

const (
	GroupStatusActive  = "active"
	GroupStatusDeleted = "deleted"
)

// ConnectionGroup 连接分组，对应表 connection_groups
type ConnectionGroup struct {
	ID        string // 主键，手工随机串（无 AUTO_INCREMENT）
	Name      string // 分组显示名
	Status    string // 状态：active / deleted（字符串）
	CreatedAt int64  // 创建时间 Unix 秒
}

// Connection 表示一条 SSH 连接配置记录，对应表 connections
type Connection struct {
	ID        string // 主键 ID，手工生成的随机字符串
	Name      string // 连接名称（可选）
	Host      string // 主机地址或 IP
	Port      int    // 端口号
	User      string // 用户名
	Password  string // 密码
	GroupID   string // 关联 connection_groups.id，空表示未分组
	GroupName string // 展示用：JOIN connection_groups.name，不落库
	Enabled   int    // 是否启用：1 启用，0 禁用
	Deleted   int    // 是否逻辑删除：1 删除，0 正常
	CreatedAt int64  // 创建时间（Unix 时间戳）
}

const connSelectSQL = `
SELECT c.id, c.name, c.host, c.port, c.user, c.password,
       IFNULL(c.group_id,''), IFNULL(g.name,''),
       c.enabled, c.deleted, c.created_at
FROM connections c
LEFT JOIN connection_groups g ON g.id=c.group_id`

// defaultDBPath 返回默认数据库文件路径：与可执行文件同目录下的 data.db
func defaultDBPath() string {
	if p, err := os.Executable(); err == nil { // 获取当前可执行文件路径
		return filepath.Join(filepath.Dir(p), "data.db") // 位于同目录 data.db
	}
	return "data.db" // 兜底：如果无法获取可执行路径，则使用当前工作目录
}

// Open 打开指定路径的 SQLite 数据库，如果路径为空则使用默认路径
func Open(path string) (*sql.DB, error) {
	if path == "" { // 未指定路径时
		path = defaultDBPath() // 使用默认 data.db 路径
	}
	dsn := "file:" + path          // SQLite DSN，使用 file: 前缀
	return sql.Open("sqlite", dsn) // 通过 modernc.org/sqlite 打开数据库
}

// InitSchema 初始化两张表：connection_groups（分组）+ connections（SSH 连接）
func InitSchema(db *sql.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS connection_groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS connections (
  id TEXT PRIMARY KEY,
  name TEXT,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  user TEXT NOT NULL,
  password TEXT NOT NULL,
  group_id TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  deleted INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
`
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	if err := ensureColumn(db, "connection_groups", "status", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
		return err
	}
	if err := ensureColumn(db, "connections", "group_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "connections", "deleted", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS idx_connections_group_name_active`); err != nil {
		return err
	}
	if _, err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_connection_groups_name_active
  ON connection_groups(name) WHERE status='active';
`); err != nil {
		return err
	}
	if err := migrateToTwoTables(db); err != nil {
		return err
	}
	return dropColumn(db, "connections", "group_name")
}

func tableExists(db *sql.DB, name string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return n > 0, err
}

func columnExists(db *sql.DB, table, col string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, col).Scan(&n)
	return n > 0, err
}

// migrateToTwoTables 把单表时期的 group 行、以及历史 group_name 收进 connection_groups
func migrateToTwoTables(db *sql.DB) error {
	hasItemType, err := columnExists(db, "connections", "item_type")
	if err != nil {
		return err
	}
	if hasItemType {
		if err := moveEmbeddedGroups(db); err != nil {
			return err
		}
	}
	return migrateGroupsFromNames(db)
}

func moveEmbeddedGroups(db *sql.DB) error {
	hasStatus, err := columnExists(db, "connections", "status")
	if err != nil {
		return err
	}
	q := `SELECT id,name,created_at FROM connections WHERE item_type='group'`
	if hasStatus {
		q = `SELECT id,name,IFNULL(status,'active'),created_at FROM connections WHERE item_type='group'`
	}
	rows, err := db.Query(q)
	if err != nil {
		return err
	}
	type embedded struct {
		id, name, status string
		createdAt        int64
	}
	var groups []embedded
	for rows.Next() {
		var g embedded
		g.status = GroupStatusActive
		if hasStatus {
			if err := rows.Scan(&g.id, &g.name, &g.status, &g.createdAt); err != nil {
				rows.Close()
				return err
			}
		} else if err := rows.Scan(&g.id, &g.name, &g.createdAt); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, g := range groups {
		name := strings.TrimSpace(g.name)
		if name == "" || name == "未分组" {
			continue
		}
		status := strings.TrimSpace(g.status)
		if status != GroupStatusActive && status != GroupStatusDeleted {
			status = GroupStatusActive
		}
		if _, err := getActiveGroupByName(db, name); err == nil {
			continue
		} else if err != sql.ErrNoRows {
			return err
		}
		id := g.id
		var taken int
		if err := db.QueryRow(`SELECT COUNT(*) FROM connection_groups WHERE id=?`, id).Scan(&taken); err != nil {
			return err
		}
		if taken > 0 {
			id = NewID()
		}
		if g.createdAt <= 0 {
			g.createdAt = time.Now().Unix()
		}
		if _, err := db.Exec(
			`INSERT INTO connection_groups(id,name,status,created_at) VALUES(?,?,?,?)`,
			id, name, status, g.createdAt,
		); err != nil {
			return err
		}
	}
	_, err = db.Exec(`DELETE FROM connections WHERE item_type='group'`)
	return err
}

func migrateGroupsFromNames(db *sql.DB) error {
	hasName, err := columnExists(db, "connections", "group_name")
	if err != nil {
		return err
	}
	if !hasName {
		return nil
	}
	rows, err := db.Query(`SELECT DISTINCT TRIM(group_name) FROM connections WHERE group_name IS NOT NULL AND TRIM(group_name)!=''`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, name := range names {
		g, err := EnsureGroupByName(db, name)
		if err != nil {
			if name == "未分组" {
				continue
			}
			return err
		}
		if _, err := db.Exec(
			`UPDATE connections SET group_id=? WHERE TRIM(IFNULL(group_name,''))=? AND (group_id IS NULL OR group_id='')`,
			g.ID, name,
		); err != nil {
			return err
		}
	}
	return nil
}

func scanGroup(row interface{ Scan(dest ...any) error }) (ConnectionGroup, error) {
	var g ConnectionGroup
	err := row.Scan(&g.ID, &g.Name, &g.Status, &g.CreatedAt)
	return g, err
}

func getActiveGroupByName(db *sql.DB, name string) (ConnectionGroup, error) {
	row := db.QueryRow(
		`SELECT id,name,status,created_at FROM connection_groups WHERE name=? AND status=? LIMIT 1`,
		name, GroupStatusActive,
	)
	return scanGroup(row)
}

func validateGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("分组名不能为空")
	}
	if name == "未分组" {
		return "", fmt.Errorf("不能使用「未分组」作为分组名")
	}
	return name, nil
}

// EnsureGroupByName 按名称获取或创建活跃分组
func EnsureGroupByName(db *sql.DB, name string) (ConnectionGroup, error) {
	name, err := validateGroupName(name)
	if err != nil {
		return ConnectionGroup{}, err
	}
	g, err := getActiveGroupByName(db, name)
	if err == nil {
		return g, nil
	}
	if err != sql.ErrNoRows {
		return ConnectionGroup{}, err
	}
	g = ConnectionGroup{
		ID:        NewID(),
		Name:      name,
		Status:    GroupStatusActive,
		CreatedAt: time.Now().Unix(),
	}
	_, err = db.Exec(
		`INSERT INTO connection_groups(id,name,status,created_at) VALUES(?,?,?,?)`,
		g.ID, g.Name, g.Status, g.CreatedAt,
	)
	return g, err
}

// ListGroups 列出分组（默认仅 active）
func ListGroups(db *sql.DB, includeDeleted bool) ([]ConnectionGroup, error) {
	q := `SELECT id,name,status,created_at FROM connection_groups`
	var args []interface{}
	if !includeDeleted {
		q += ` WHERE status=?`
		args = append(args, GroupStatusActive)
	}
	q += ` ORDER BY name ASC`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConnectionGroup
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if out == nil {
		out = []ConnectionGroup{}
	}
	return out, rows.Err()
}

// InsertGroup 新建分组
func InsertGroup(db *sql.DB, name string) (ConnectionGroup, error) {
	name, err := validateGroupName(name)
	if err != nil {
		return ConnectionGroup{}, err
	}
	if _, err := getActiveGroupByName(db, name); err == nil {
		return ConnectionGroup{}, fmt.Errorf("分组「%s」已存在", name)
	} else if err != sql.ErrNoRows {
		return ConnectionGroup{}, err
	}
	return EnsureGroupByName(db, name)
}

// RenameGroup 重命名分组：只改 connection_groups，连接通过 group_id 关联
func RenameGroup(db *sql.DB, groupID, newName string) error {
	newName, err := validateGroupName(newName)
	if err != nil {
		return err
	}
	if groupID == "" {
		return fmt.Errorf("缺少分组 ID")
	}
	var oldName string
	err = db.QueryRow(
		`SELECT name FROM connection_groups WHERE id=? AND status=?`,
		groupID, GroupStatusActive,
	).Scan(&oldName)
	if err == sql.ErrNoRows {
		return fmt.Errorf("分组不存在")
	}
	if err != nil {
		return err
	}
	if oldName == newName {
		return nil
	}
	var otherID string
	err = db.QueryRow(
		`SELECT id FROM connection_groups WHERE name=? AND status=? AND id!=? LIMIT 1`,
		newName, GroupStatusActive, groupID,
	).Scan(&otherID)
	if err == nil {
		return fmt.Errorf("分组「%s」已存在", newName)
	}
	if err != sql.ErrNoRows {
		return err
	}
	res, err := db.Exec(`UPDATE connection_groups SET name=? WHERE id=? AND status=?`, newName, groupID, GroupStatusActive)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("分组不存在")
	}
	return nil
}

// RenameGroupByName 按旧名称重命名
func RenameGroupByName(db *sql.DB, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	if oldName == "" || oldName == "未分组" {
		return fmt.Errorf("该分组不可重命名")
	}
	g, err := getActiveGroupByName(db, oldName)
	if err == sql.ErrNoRows {
		g, err = EnsureGroupByName(db, oldName)
	}
	if err != nil {
		return err
	}
	return RenameGroup(db, g.ID, newName)
}

// DeleteGroupByName 删除分组：组内连接移到未分组，分组标记 deleted
func DeleteGroupByName(db *sql.DB, name string) (moved int, err error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "未分组" {
		return 0, fmt.Errorf("该分组不可删除")
	}
	g, err := getActiveGroupByName(db, name)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("分组不存在")
	}
	if err != nil {
		return 0, err
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.Exec(`UPDATE connections SET group_id='' WHERE group_id=?`, g.ID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if _, err := tx.Exec(`UPDATE connection_groups SET status=? WHERE id=?`, GroupStatusDeleted, g.ID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

// AssignUngroupedToGroup 把未分组连接归入指定分组（可新建）
func AssignUngroupedToGroup(db *sql.DB, newName string) (moved int, err error) {
	g, err := EnsureGroupByName(db, newName)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(`UPDATE connections SET group_id=? WHERE group_id IS NULL OR TRIM(group_id)=''`, g.ID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func resolveGroupFields(db *sql.DB, groupName string) (groupID, name string, err error) {
	name = strings.TrimSpace(groupName)
	if name == "" || name == "未分组" {
		return "", "", nil
	}
	g, err := EnsureGroupByName(db, name)
	if err != nil {
		return "", "", err
	}
	return g.ID, g.Name, nil
}

func scanConnection(row interface{ Scan(dest ...any) error }) (Connection, error) {
	var c Connection
	err := row.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.User, &c.Password, &c.GroupID, &c.GroupName, &c.Enabled, &c.Deleted, &c.CreatedAt)
	return c, err
}

// InsertConnection 往数据库中插入一条连接配置记录
func InsertConnection(db *sql.DB, c Connection) error {
	gid, gname, err := resolveGroupFields(db, c.GroupName)
	if err != nil {
		return err
	}
	if c.GroupID != "" {
		gid = c.GroupID
	}
	c.GroupID, c.GroupName = gid, gname
	q := `INSERT INTO connections(id,name,host,port,user,password,group_id,enabled,deleted,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`
	_, err = db.Exec(q, c.ID, c.Name, c.Host, c.Port, c.User, c.Password, c.GroupID, c.Enabled, c.Deleted, c.CreatedAt)
	return err
}

// ListConnections 按条件查询连接列表
func ListConnections(db *sql.DB, includeDeleted bool, groupFilter string) ([]Connection, error) {
	q := connSelectSQL
	var where string
	var args []interface{}
	if !includeDeleted {
		where = " c.deleted=0"
	}
	if groupFilter != "" {
		if where != "" {
			where += " AND"
		}
		where += " g.name=?"
		args = append(args, groupFilter)
	}
	if where != "" {
		q += " WHERE" + where
	}
	q += " ORDER BY c.created_at DESC"
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	if res == nil {
		res = []Connection{}
	}
	return res, rows.Err()
}

// NewID 生成一个随机 ID，用于主键
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex(b)
}

func hex(b []byte) string {
	const hexdigits = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = hexdigits[v>>4]
		dst[i*2+1] = hexdigits[v&0x0f]
	}
	return string(dst)
}

func ensureColumn(db *sql.DB, table, col, def string) error {
	exists, err := columnExists(db, table, col)
	if err != nil {
		return err
	}
	if !exists {
		_, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + def)
		return err
	}
	return nil
}

func dropColumn(db *sql.DB, table, col string) error {
	exists, err := columnExists(db, table, col)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` DROP COLUMN ` + col)
	return err
}

// SetDeleted 设置指定连接的 deleted 标记（0=正常，1=已删除）
func SetDeleted(db *sql.DB, id string, deleted int) error {
	_, err := db.Exec(`UPDATE connections SET deleted=? WHERE id=?`, deleted, id)
	return err
}

// UpdateConnection 按 ID 更新连接配置（不改 deleted、created_at）
func UpdateConnection(db *sql.DB, c Connection) error {
	gid, gname, err := resolveGroupFields(db, c.GroupName)
	if err != nil {
		return err
	}
	c.GroupID, c.GroupName = gid, gname
	res, err := db.Exec(
		`UPDATE connections SET name=?, host=?, port=?, user=?, password=?, group_id=?, enabled=? WHERE id=?`,
		c.Name, c.Host, c.Port, c.User, c.Password, c.GroupID, c.Enabled, c.ID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("连接不存在")
	}
	return nil
}

// MoveConnection 把连接移到指定分组；groupName 为空或「未分组」则取消分组
func MoveConnection(db *sql.DB, id, groupName string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("缺少连接 ID")
	}
	if _, err := GetByID(db, id); err != nil {
		return fmt.Errorf("连接不存在")
	}
	gid, _, err := resolveGroupFields(db, groupName)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE connections SET group_id=? WHERE id=?`, gid, id)
	return err
}

// GetByID 根据 ID 查询一条连接配置记录
func GetByID(db *sql.DB, id string) (Connection, error) {
	row := db.QueryRow(connSelectSQL+` WHERE c.id=?`, id)
	return scanConnection(row)
}
