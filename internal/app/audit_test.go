package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func logsForTest(t *testing.T, h *harness) []OperationLog {
	t.Helper()
	var rows []OperationLog
	if err := h.app.Store.DB.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}
func TestOperationLogsAuthenticationAndSecrets(t *testing.T) {
	h := newHarness(t)
	status, _, _ := h.call(t, "GET", "/api/admin/operation-logs", nil, false, nil)
	expect(t, status, 401)
	h.setup(t)
	password := "do-not-log-this-password"
	status, _, _ = h.call(t, "POST", "/api/admin/login", map[string]string{"username": "admin", "password": password}, false, nil)
	expect(t, status, 401)
	rows := logsForTest(t, h)
	last := rows[len(rows)-1]
	if last.Action != "auth.login" || last.Authenticated || last.Success || last.Username != "admin" || last.Status != 401 {
		t.Fatal(last)
	}
	status, _, _ = h.call(t, "POST", "/api/admin/login", map[string]string{"username": "admin", "password": "go-test-password-123"}, false, nil)
	expect(t, status, 200)
	rows = logsForTest(t, h)
	last = rows[len(rows)-1]
	if !last.Authenticated || !last.Success {
		t.Fatal(last)
	}
	raw, _ := json.Marshal(rows)
	for _, secret := range []string{password, "go-test-password-123", h.csrf, "ytl_session", "scrypt"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("sensitive value in logs")
		}
	}
	// CSRF-denied changes are logged as failures, never executed.
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]string{"name": "blocked-change"}, true, map[string]string{"X-CSRF-Token": "wrong"})
	expect(t, status, 403)
	rows = logsForTest(t, h)
	last = rows[len(rows)-1]
	if last.Success || last.Status != 403 {
		t.Fatal(last)
	}
	status, _, _ = h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	if len(logsForTest(t, h)) != len(rows) {
		t.Fatal("public reads must not be audited")
	}
}
func TestOperationLogsCaseLifecycleAndDeletedNames(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	c := contentCase(t, h, "life-forward")
	c.Title = "日志案例"
	c.Status = "draft"
	c.Hero = false
	status, value, _ := h.call(t, "POST", "/api/admin/cases", c, true, nil)
	expect(t, status, 201)
	c.ID = value["id"].(string)
	c.Status = "published"
	status, _, _ = h.call(t, "PUT", "/api/admin/cases/"+c.ID, c, true, nil)
	expect(t, status, 200)
	c.Status = "archived"
	status, _, _ = h.call(t, "PUT", "/api/admin/cases/"+c.ID, c, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/cases/"+c.ID, nil, true, nil)
	expect(t, status, 200)
	rows := logsForTest(t, h)
	actions := []string{}
	for _, r := range rows {
		if r.TargetID == c.ID {
			actions = append(actions, r.Action)
			if r.Target != "日志案例" || !r.Authenticated || r.Username != "admin" || !r.Success {
				t.Fatal(r)
			}
		}
	}
	if strings.Join(actions, ",") != "case.create,case.publish,case.archive,case.delete" {
		t.Fatal(actions)
	}
	c = contentCase(t, h, "life-forward")
	status, _, _ = h.call(t, "DELETE", "/api/admin/categories/"+c.CategoryID, nil, true, nil)
	expect(t, status, 409)
	rows = logsForTest(t, h)
	last := rows[len(rows)-1]
	if last.Success || last.Target == "" || last.Action != "categories.delete" {
		t.Fatal(last)
	}
}
func TestOperationLogCompanyAndTaxonomy(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	c, err := getRow[Company](h.app.Store.DB, "settings", "company")
	if err != nil {
		t.Fatal(err)
	}
	c.TeamMembers[0].Wechat = "private-contact-value"
	status, _, _ := h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": c.TeamMembers}, true, nil)
	expect(t, status, 200)
	rows := logsForTest(t, h)
	last := rows[len(rows)-1]
	if !strings.Contains(last.Target, c.TeamMembers[0].Name) {
		t.Fatal(last)
	}
	raw, _ := json.Marshal(rows)
	if strings.Contains(string(raw), "private-contact-value") {
		t.Fatal("contact details leaked")
	}
	status, v, _ := h.call(t, "POST", "/api/admin/clients", Client{Name: "日志客户", Order: 10}, true, nil)
	expect(t, status, 201)
	id := v["id"].(string)
	status, _, _ = h.call(t, "DELETE", "/api/admin/clients/"+id, nil, true, nil)
	expect(t, status, 200)
	rows = logsForTest(t, h)
	if rows[len(rows)-1].Target != "日志客户" {
		t.Fatal(rows)
	}
	status, _, _ = h.call(t, "GET", "/api/admin/export", nil, true, nil)
	expect(t, status, 200)
	rows = logsForTest(t, h)
	if rows[len(rows)-1].Action != "content.export" {
		t.Fatal(rows)
	}
}
func TestOperationLogFiltersPaginationAndPersistence(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	base := time.Date(2026, 10, 28, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 55; i++ {
		row := OperationLog{CreatedAt: base.Add(time.Duration(i) * time.Minute).UnixMilli(), Action: "case.update", Target: "example", Success: i%2 == 0, Status: 200}
		if err := h.app.Store.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/admin/operation-logs?action=case.update&from=2026-10-28T08:00:00Z&to=2026-10-28T09:00:00Z"
	status, v, _ := h.call(t, "GET", path, nil, true, nil)
	expect(t, status, 200)
	items := v["items"].([]any)
	if v["total"] != float64(55) || len(items) != 50 {
		t.Fatal(v)
	}
	first := items[0].(map[string]any)["id"].(float64)
	status, v, _ = h.call(t, "GET", path+"&page=2", nil, true, nil)
	expect(t, status, 200)
	if len(v["items"].([]any)) != 5 || v["items"].([]any)[0].(map[string]any)["id"].(float64) >= first {
		t.Fatal(v)
	}
	status, v, _ = h.call(t, "GET", path+"&result=failure", nil, true, nil)
	expect(t, status, 200)
	if v["total"] != float64(27) {
		t.Fatal(v)
	}
	for _, q := range []string{"page=-1", "page=NaN", "action=sql-injection", "result=other", "from=bad", "from=2026-10-29T00:00:00Z&to=2026-10-28T00:00:00Z"} {
		status, _, _ = h.call(t, "GET", "/api/admin/operation-logs?"+q, nil, true, nil)
		expect(t, status, 400)
	}
	other, err := OpenStore(h.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var n int64
	other.DB.Model(&OperationLog{}).Count(&n)
	if n != 56 {
		t.Fatal(n)
	}
}
func TestOperationLogProxyIPTrust(t *testing.T) {
	for _, tc := range []struct {
		production       bool
		peer, real, want string
	}{
		{true, "127.0.0.1:1234", "203.0.113.8", "203.0.113.8"},
		{true, "[::1]:1234", "2001:db8::1", "2001:db8::1"},
		{true, "198.51.100.2:1234", "203.0.113.8", "198.51.100.2"},
		{false, "127.0.0.1:1234", "203.0.113.8", "127.0.0.1"},
		{true, "127.0.0.1:1234", "invalid,203.0.113.8", "127.0.0.1"},
	} {
		r := httptest.NewRequest("GET", "http://localhost/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Real-IP", tc.real)
		r.Header.Set("X-Forwarded-For", "192.0.2.66")
		a := Server{cfg: Config{Production: tc.production}}
		if got := a.auditClientIP(r); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}

func TestOperationLogUploadsLogoutAndPassword(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	status, v, _ := h.call(t, "POST", "/api/admin/uploads", []byte("\x89PNG\r\n\x1a\n"+strings.Repeat("x", 64)), true, map[string]string{"X-File-Name": "audit-test.png"})
	expect(t, status, 201)
	id := v["id"].(string)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+id, nil, true, nil)
	expect(t, status, 200)
	rows := logsForTest(t, h)
	if rows[len(rows)-1].Target != "audit-test.png" || rows[len(rows)-1].Action != "upload.delete" {
		t.Fatal(rows)
	}
	status, _, _ = h.call(t, "POST", "/api/admin/password", map[string]string{"currentPassword": "wrong-password-123", "password": "new-secret-password-123"}, true, nil)
	expect(t, status, 400)
	rows = logsForTest(t, h)
	last := rows[len(rows)-1]
	if last.Action != "auth.password" || last.Success {
		t.Fatal(last)
	}
	status, _, _ = h.call(t, "POST", "/api/admin/logout", map[string]string{}, true, nil)
	expect(t, status, 200)
	rows = logsForTest(t, h)
	last = rows[len(rows)-1]
	if last.Action != "auth.logout" || !last.Authenticated || last.Username != "admin" || !last.Success {
		t.Fatal(last)
	}
	status, _, _ = h.call(t, "GET", "/api/admin/operation-logs", nil, true, nil)
	expect(t, status, 401)
}
