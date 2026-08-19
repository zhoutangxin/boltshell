package db // 数据库访问层：封装 SQLite 相关操作

import (
	"crypto/rand"          // 标准库：安全随机数，用于生成 ID
	"database/sql"         // 标准库：通用数据库接口
	"fmt"                  // 标准库：错误信息
	_ "modernc.org/sqlite" // 第三方：纯 Go 的 SQLite 驱动（仅导入以注册驱动）
	"os"                   // 标准库：可执行文件路径获取
	"path/filepath"        // 标准库：路径拼接
)

// Connection 表示一条 SSH 连接配置记录，对应表 connections
type Connection struct {
	ID        string // 主键 ID，手工生成的随机字符串
	Name      string // 连接名称（可选）
	Host      string // 主机地址或 IP
	Port      int    // 端口号
	User      string // 用户名
	Password  string // 密码
	GroupName string // 分组名（可为空）
	Enabled   int    // 是否启用：1 启用，0 禁用
	Deleted   int    // 是否逻辑删除：1 删除，0 正常
	CreatedAt int64  // 创建时间（Unix 时间戳）
}

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

// InitSchema 初始化数据库表结构，如果表不存在则创建；老版本表结构会自动补充字段
func InitSchema(db *sql.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS connections (
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
`
	_, err := db.Exec(ddl) // 执行建表语句
	if err != nil {
		return err
	}
	// 为兼容老版本，确保新字段存在（如果老库里没有这些列则自动 ALTER TABLE 添加）
	if err := ensureColumn(db, "connections", "group_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "connections", "deleted", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

// InsertConnection 往数据库中插入一条连接配置记录
func InsertConnection(db *sql.DB, c Connection) error {
	q := `INSERT INTO connections(id,name,host,port,user,password,group_name,enabled,deleted,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`
	_, err := db.Exec(q, c.ID, c.Name, c.Host, c.Port, c.User, c.Password, c.GroupName, c.Enabled, c.Deleted, c.CreatedAt)
	return err
}

// ListConnections 按条件查询连接列表，可选择是否包含已删除、以及按分组过滤
func ListConnections(db *sql.DB, includeDeleted bool, groupFilter string) ([]Connection, error) {
	q := `SELECT id,name,host,port,user,password,group_name,enabled,deleted,created_at FROM connections` // 基本查询
	var where string
	var args []interface{}
	if !includeDeleted { // 不包含已删除的记录时
		where = " deleted=0" // 增加 deleted=0 条件
	}
	if groupFilter != "" { // 如果设置了分组过滤
		if where != "" {
			where += " AND"
		}
		where += " group_name=?" // 过滤指定分组
		args = append(args, groupFilter)
	}
	if where != "" {
		q += " WHERE" + where // 添加 WHERE 子句
	}
	q += " ORDER BY created_at DESC"  // 按创建时间倒序展示
	rows, err := db.Query(q, args...) // 执行查询
	if err != nil {
		return nil, err
	}
	defer rows.Close() // 用完 rows 要关闭
	var res []Connection
	for rows.Next() { // 逐行扫描
		var c Connection
		if err := rows.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.User, &c.Password, &c.GroupName, &c.Enabled, &c.Deleted, &c.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, rows.Err() // 返回结果以及 rows 的最终错误（例如遍历过程中的错误）
}

// NewID 生成一个随机 ID，用于 Connection 的主键
func NewID() string {
	b := make([]byte, 16) // 16 字节随机数
	_, _ = rand.Read(b)   // 忽略错误，足够用于生成 ID
	return hex(b)         // 转为十六进制字符串
}

// hex 把字节切片转换为小写十六进制字符串
func hex(b []byte) string {
	const hexdigits = "0123456789abcdef"
	dst := make([]byte, len(b)*2) // 每个字节对应两个十六进制字符
	for i, v := range b {
		dst[i*2] = hexdigits[v>>4]     // 高 4 位
		dst[i*2+1] = hexdigits[v&0x0f] // 低 4 位
	}
	return string(dst)
}

// ensureColumn 确保某个表里存在指定列，如果不存在就执行 ALTER TABLE 添加
func ensureColumn(db *sql.DB, table, col, def string) error {
	var exists int
	// pragma_table_info(?) 返回表结构信息，这里统计名为 col 的列个数
	row := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, col)
	if err := row.Scan(&exists); err != nil { // 读取查询结果
		return err
	}
	if exists == 0 { // 如果列不存在
		_, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + def) // 动态拼接 ALTER 语句
		return err
	}
	return nil
}

// SetDeleted 设置指定连接的 deleted 标记（0=正常，1=已删除）
func SetDeleted(db *sql.DB, id string, deleted int) error {
	_, err := db.Exec(`UPDATE connections SET deleted=? WHERE id=?`, deleted, id)
	return err
}

// UpdateConnection 按 ID 更新连接配置（不改 deleted、created_at）
func UpdateConnection(db *sql.DB, c Connection) error {
	res, err := db.Exec(
		`UPDATE connections SET name=?, host=?, port=?, user=?, password=?, group_name=?, enabled=? WHERE id=?`,
		c.Name, c.Host, c.Port, c.User, c.Password, c.GroupName, c.Enabled, c.ID,
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

// GetByID 根据 ID 查询一条连接配置记录
func GetByID(db *sql.DB, id string) (Connection, error) {
	var c Connection
	row := db.QueryRow(`SELECT id,name,host,port,user,password,group_name,enabled,deleted,created_at FROM connections WHERE id=?`, id)
	err := row.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.User, &c.Password, &c.GroupName, &c.Enabled, &c.Deleted, &c.CreatedAt)
	return c, err
}
