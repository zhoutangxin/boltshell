package db

import (
	"crypto/rand"
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type Connection struct {
	ID        string
	Name      string
	Host      string
	Port      int
	User      string
	Password  string
	Enabled   int
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
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL
);
`
	_, err := db.Exec(ddl)
	return err
}

func InsertConnection(db *sql.DB, c Connection) error {
	q := `INSERT INTO connections(id,name,host,port,user,password,enabled,created_at) VALUES(?,?,?,?,?,?,?,?)`
	_, err := db.Exec(q, c.ID, c.Name, c.Host, c.Port, c.User, c.Password, c.Enabled, c.CreatedAt)
	return err
}

func ListConnections(db *sql.DB) ([]Connection, error) {
	rows, err := db.Query(`SELECT id,name,host,port,user,password,enabled,created_at FROM connections ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.User, &c.Password, &c.Enabled, &c.CreatedAt); err != nil {
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
