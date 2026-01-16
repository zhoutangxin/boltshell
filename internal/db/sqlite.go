package db

import (
	"crypto/rand"
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

type Connection struct {
	ID        string
	Name      string
	Host      string
	Port      int
	User      string
	Password  string
	GroupName string
	Enabled   int
	Deleted   int
	CreatedAt int64
}

func defaultDBPath() string {
	if p, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(p), "data.db")
	}
	return "data.db"
}

func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = defaultDBPath()
	}
	dsn := "file:" + path
	return sql.Open("sqlite", dsn)
}

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
	_, err := db.Exec(ddl)
	if err != nil {
		return err
	}
	if err := ensureColumn(db, "connections", "group_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureColumn(db, "connections", "deleted", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func InsertConnection(db *sql.DB, c Connection) error {
	q := `INSERT INTO connections(id,name,host,port,user,password,group_name,enabled,deleted,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`
	_, err := db.Exec(q, c.ID, c.Name, c.Host, c.Port, c.User, c.Password, c.GroupName, c.Enabled, c.Deleted, c.CreatedAt)
	return err
}

func ListConnections(db *sql.DB, includeDeleted bool, groupFilter string) ([]Connection, error) {
	q := `SELECT id,name,host,port,user,password,group_name,enabled,deleted,created_at FROM connections`
	var where string
	var args []interface{}
	if !includeDeleted {
		where = " deleted=0"
	}
	if groupFilter != "" {
		if where != "" {
			where += " AND"
		}
		where += " group_name=?"
		args = append(args, groupFilter)
	}
	if where != "" {
		q += " WHERE" + where
	}
	q += " ORDER BY created_at DESC"
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.User, &c.Password, &c.GroupName, &c.Enabled, &c.Deleted, &c.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, c)
	}
	return res, rows.Err()
}

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
	var exists int
	row := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, col)
	if err := row.Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		_, err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + col + ` ` + def)
		return err
	}
	return nil
}

func SetDeleted(db *sql.DB, id string, deleted int) error {
	_, err := db.Exec(`UPDATE connections SET deleted=? WHERE id=?`, deleted, id)
	return err
}

func GetByID(db *sql.DB, id string) (Connection, error) {
	var c Connection
	row := db.QueryRow(`SELECT id,name,host,port,user,password,group_name,enabled,deleted,created_at FROM connections WHERE id=?`, id)
	err := row.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.User, &c.Password, &c.GroupName, &c.Enabled, &c.Deleted, &c.CreatedAt)
	return c, err
}
