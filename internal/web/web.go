package web

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"database/sql"
	"ssh-go/internal/db"
	"ssh-go/internal/logging"
)

func Start(addr string, database *sql.DB, logger *logging.Logger) error {
	indexT := template.Must(template.New("index").Parse(indexHTML))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		list, err := db.ListConnections(database)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = indexT.Execute(w, struct {
			Items []db.Connection
		}{Items: list})
	})
	http.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		name := r.Form.Get("name")
		host := r.Form.Get("host")
		user := r.Form.Get("user")
		pass := r.Form.Get("pass")
		portStr := r.Form.Get("port")
		enabledStr := r.Form.Get("enabled")
		if host == "" || user == "" || pass == "" {
			http.Error(w, "缺少必填项", 400)
			return
		}
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			port = 22
		}
		enabled := 1
		if enabledStr == "0" {
			enabled = 0
		}
		err := db.InsertConnection(database, db.Connection{
			ID:        db.NewID(),
			Name:      name,
			Host:      host,
			Port:      port,
			User:      user,
			Password:  pass,
			Enabled:   enabled,
			CreatedAt: time.Now().Unix(),
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, "/", 302)
	})
	logger.Info("HTTP: %s", addr)
	return http.ListenAndServe(addr, nil)
}

const indexHTML = `
<!doctype html>
<html>
<head>
<meta charset="utf-8"/>
<title>连接管理</title>
<style>
body{font-family:system-ui,Segoe UI,Arial;margin:24px;}
table{border-collapse:collapse;width:100%;margin-top:16px;}
th,td{border:1px solid #ddd;padding:8px;text-align:left;}
th{background:#f3f3f3;}
input,select{padding:6px;margin:4px 0;width:100%;}
.row{display:grid;grid-template-columns:repeat(2,1fr);gap:12px;}
.actions{margin-top:12px}
button{padding:8px 12px}
</style>
</head>
<body>
<h2>添加连接</h2>
<form method="post" action="/save">
  <div class="row">
    <div><label>名称</label><input name="name"/></div>
    <div><label>主机</label><input name="host" required/></div>
    <div><label>端口</label><input name="port" value="22"/></div>
    <div><label>用户名</label><input name="user" required/></div>
    <div><label>密码</label><input name="pass" type="password" required/></div>
    <div><label>启用</label>
      <select name="enabled">
        <option value="1">启用</option>
        <option value="0">禁用</option>
      </select>
    </div>
  </div>
  <div class="actions"><button type="submit">保存</button></div>
</form>

<h2>连接列表</h2>
<table>
  <thead><tr><th>名称</th><th>主机</th><th>端口</th><th>用户</th><th>启用</th><th>创建时间</th></tr></thead>
  <tbody>
    {{range .Items}}
      <tr>
        <td>{{.Name}}</td>
        <td>{{.Host}}</td>
        <td>{{.Port}}</td>
        <td>{{.User}}</td>
        <td>{{.Enabled}}</td>
        <td>{{.CreatedAt}}</td>
      </tr>
    {{end}}
  </tbody>
</table>
</body>
</html>
`
