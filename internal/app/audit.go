package app

import (
	"context"
	"log"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// OperationLog contains only allowlisted metadata, never request bodies or credentials.
type OperationLog struct {
	ID            uint64 `json:"id" gorm:"primaryKey"`
	CreatedAt     int64  `json:"createdAt" gorm:"index"`
	Username      string `json:"username"`
	Authenticated bool   `json:"authenticated"`
	IP            string `json:"ip"`
	Action        string `json:"action" gorm:"index"`
	TargetID      string `json:"targetId"`
	Target        string `json:"target"`
	Success       bool   `json:"success"`
	Status        int    `json:"status"`
}

type auditKey struct{}
type auditMetadata struct {
	Action, TargetID, Target, Username string
	Authenticated                      bool
}

var auditActions = map[string]string{
	"auth.setup": "初始化管理员", "auth.login": "登录", "auth.logout": "退出登录", "auth.password": "修改密码",
	"case.create": "新增案例", "case.update": "修改案例", "case.publish": "发布案例", "case.archive": "下架案例", "case.delete": "删除案例",
	"categories.create": "新增业务分类", "categories.update": "修改业务分类", "categories.delete": "删除业务分类",
	"clients.create": "新增客户", "clients.update": "修改客户", "clients.delete": "删除客户",
	"company.update": "修改公司与名片", "platforms.update": "修改平台入口", "content.export": "导出内容",
	"upload.create": "上传素材", "upload.delete": "删除素材",
}

func auditAction(pattern string) string {
	switch pattern {
	case "POST /api/admin/setup":
		return "auth.setup"
	case "POST /api/admin/login":
		return "auth.login"
	case "POST /api/admin/logout":
		return "auth.logout"
	case "POST /api/admin/password":
		return "auth.password"
	case "PUT /api/admin/company":
		return "company.update"
	case "PUT /api/admin/platforms":
		return "platforms.update"
	case "GET /api/admin/export":
		return "content.export"
	case "POST /api/admin/uploads":
		return "upload.create"
	case "DELETE /api/admin/uploads/{id}":
		return "upload.delete"
	}
	for _, resource := range []string{"cases", "categories", "clients"} {
		prefix := resource
		if resource == "cases" {
			prefix = "case"
		}
		if pattern == "POST /api/admin/"+resource {
			return prefix + ".create"
		}
		if pattern == "PUT /api/admin/"+resource+"/{id}" {
			return prefix + ".update"
		}
		if pattern == "DELETE /api/admin/"+resource+"/{id}" {
			return prefix + ".delete"
		}
	}
	return ""
}
func auditText(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	chars := []rune(strings.TrimSpace(s))
	if len(chars) > max {
		chars = chars[:max]
	}
	return string(chars)
}
func setAuditTarget(r *http.Request, id, name string) {
	if m, ok := r.Context().Value(auditKey{}).(*auditMetadata); ok {
		m.TargetID = auditText(id, 100)
		m.Target = auditText(name, 240)
	}
}

// Only our production loopback reverse proxy may supply X-Real-IP. Ignore X-Forwarded-For.
func (a *Server) auditClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return ""
	}
	if a.cfg.Production && peer.IsLoopback() {
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	return peer.String()
}
func (a *Server) auditTargetBefore(r *http.Request) {
	id := r.PathValue("id")
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/admin/cases/"):
		if x, err := getRow[Case](a.Store.DB, "cases", id); err == nil {
			setAuditTarget(r, x.ID, x.Title)
		}
	case strings.HasPrefix(r.URL.Path, "/api/admin/categories/"):
		if x, err := getRow[Category](a.Store.DB, "categories", id); err == nil {
			setAuditTarget(r, x.ID, x.Name)
		}
	case strings.HasPrefix(r.URL.Path, "/api/admin/clients/"):
		if x, err := getRow[Client](a.Store.DB, "clients", id); err == nil {
			setAuditTarget(r, x.ID, x.Name)
		}
	case strings.HasPrefix(r.URL.Path, "/api/admin/uploads/"):
		var x Upload
		if a.Store.DB.Where("id = ?", id).Take(&x).Error == nil {
			setAuditTarget(r, x.ID, x.OriginalName)
		}
	case r.URL.Path == "/api/admin/company":
		setAuditTarget(r, "company", "公司信息与团队名片")
	case r.URL.Path == "/api/admin/platforms":
		setAuditTarget(r, "platforms", "GEO与漫剧平台入口")
	case r.URL.Path == "/api/admin/export":
		setAuditTarget(r, "content", "内容JSON")
	}
}

type auditResponse struct {
	http.ResponseWriter
	status int
}

func (w *auditResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *auditResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *auditResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (a *Server) auditHandler(pattern string, h http.HandlerFunc) http.HandlerFunc {
	action := auditAction(pattern)
	if action == "" {
		return h
	}
	return func(w http.ResponseWriter, r *http.Request) {
		meta := &auditMetadata{Action: action, TargetID: auditText(r.PathValue("id"), 100)}
		r = r.WithContext(context.WithValue(r.Context(), auditKey{}, meta))
		response := &auditResponse{ResponseWriter: w}
		h(response, r)
		status := response.status
		if status == 0 {
			status = 200
		}
		entry := OperationLog{CreatedAt: time.Now().UnixMilli(), Username: meta.Username, Authenticated: meta.Authenticated, IP: a.auditClientIP(r), Action: meta.Action, TargetID: meta.TargetID, Target: meta.Target, Success: status >= 200 && status < 300, Status: status}
		// The business operation may already be committed; do not falsely report it failed if logging fails.
		if err := a.Store.DB.Create(&entry).Error; err != nil {
			log.Printf("operation log write failed: %v", err)
		}
	}
}
func (a *Server) operationLogs(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	page := 1
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 1000000 {
			return bad(400, "页码不正确")
		}
		page = n
	}
	query := a.Store.DB.Model(&OperationLog{})
	if v := q.Get("action"); v != "" {
		if _, ok := auditActions[v]; !ok {
			return bad(400, "操作类型不正确")
		}
		query = query.Where("action = ?", v)
	}
	if v := q.Get("result"); v != "" {
		if v != "success" && v != "failure" {
			return bad(400, "操作结果不正确")
		}
		query = query.Where("success = ?", v == "success")
	}
	var from, to time.Time
	for _, k := range []string{"from", "to"} {
		if v := q.Get(k); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return bad(400, "时间格式不正确")
			}
			if k == "from" {
				from = t
				query = query.Where("created_at >= ?", t.UnixMilli())
			} else {
				to = t
				query = query.Where("created_at < ?", t.UnixMilli())
			}
		}
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return bad(400, "开始时间需早于结束时间")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return err
	}
	items := make([]OperationLog, 0)
	if err := query.Order("id DESC").Limit(50).Offset((page - 1) * 50).Find(&items).Error; err != nil {
		return err
	}
	return respond(w, 200, map[string]any{"items": items, "total": total, "page": page, "pageSize": 50, "actions": auditActions})
}

// Login failures retain only syntactically valid account names, and never imply verified identity.
func auditLogin(r *http.Request, username string, verified bool) {
	if m, ok := r.Context().Value(auditKey{}).(*auditMetadata); ok {
		if usernamePattern.MatchString(username) {
			m.Username = username
		}
		m.Authenticated = verified
	}
}
func auditCaseStatus(r *http.Request, oldStatus, newStatus string) {
	if m, ok := r.Context().Value(auditKey{}).(*auditMetadata); ok && oldStatus != newStatus {
		if newStatus == "published" {
			m.Action = "case.publish"
		} else if newStatus == "archived" {
			m.Action = "case.archive"
		}
	}
}
func auditCompanyMembers(r *http.Request, before, after []TeamMember) {
	old := map[string]TeamMember{}
	for _, m := range before {
		old[m.ID] = m
	}
	changed := []string{}
	for _, m := range after {
		if v, ok := old[m.ID]; !ok || !reflect.DeepEqual(v, m) {
			changed = append(changed, m.Name)
		}
		delete(old, m.ID)
	}
	for _, m := range before {
		if _, ok := old[m.ID]; ok {
			changed = append(changed, m.Name)
		}
	}
	if len(changed) > 0 {
		setAuditTarget(r, "company", "公司信息；涉及成员："+strings.Join(changed, "、"))
	}
}
