package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type harness struct {
	app           *Server
	server        *httptest.Server
	client        *http.Client
	csrf          string
	root, dataDir string
}

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
func newHarness(t *testing.T) *harness {
	t.Helper()
	return newStorageHarness(t, nil)
}
func newStorageHarness(t *testing.T, files FileStorage) *harness {
	t.Helper()
	a, err := New(Config{Root: projectRoot(), DataDir: t.TempDir(), FileStorage: files})
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(a)
	jar, _ := cookiejar.New(nil)
	h := &harness{app: a, server: s, client: &http.Client{Jar: jar}, root: projectRoot(), dataDir: a.Store.DataDir}
	t.Cleanup(func() { h.server.Close(); h.app.Close() })
	return h
}
func (h *harness) call(t *testing.T, method, path string, body any, auth bool, headers map[string]string) (int, map[string]any, *http.Response) {
	t.Helper()
	var reader io.Reader
	if data, ok := body.([]byte); ok {
		reader = bytes.NewReader(data)
	} else if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != "GET" && method != "HEAD" {
		req.Header.Set("Origin", h.server.URL)
		if auth {
			req.Header.Set("X-CSRF-Token", h.csrf)
		}
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := h.client
	if !auth {
		client = &http.Client{}
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	value := make(map[string]any)
	if strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		if err = json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("invalid JSON: %s", raw)
		}
	} else {
		value["bytes"] = raw
	}
	return res.StatusCode, value, res
}
func (h *harness) setup(t *testing.T) {
	t.Helper()
	status, v, _ := h.call(t, "POST", "/api/admin/setup", map[string]string{"username": "admin", "password": "go-test-password-123"}, true, nil)
	if status != 201 {
		t.Fatalf("setup %d: %v", status, v)
	}
	h.csrf = v["csrf"].(string)
}
func expect(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("HTTP status %d, expected %d", got, want)
	}
}
func contentCase(t *testing.T, h *harness, id string) Case {
	t.Helper()
	c, err := getRow[Case](h.app.Store.DB, "cases", id)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestPublicRoutes(t *testing.T) {
	h := newHarness(t)
	status, v, _ := h.call(t, "GET", "/api/public/case-list", nil, false, nil)
	expect(t, status, 200)
	if len(v["items"].([]any)) != 6 {
		t.Fatal(v)
	}
	status, v, _ = h.call(t, "GET", "/api/public/case-list?client=icbc", nil, false, nil)
	expect(t, status, 200)
	if len(v["items"].([]any)) != 3 {
		t.Fatal(v)
	}
	for _, path := range []string{"/", "/admin", "/cases/life-forward", "/site.js", "/admin/admin.js", "/assets/life-forward.jpg"} {
		status, _, _ = h.call(t, "GET", path, nil, false, nil)
		expect(t, status, 200)
	}
	for _, path := range []string{"/.data/content.sqlite", "/internal/app/seed.json", "/api/public/cases/missing"} {
		status, _, _ = h.call(t, "GET", path, nil, false, nil)
		expect(t, status, 404)
	}
	status, _, _ = h.call(t, "GET", "/api/admin/config", nil, false, nil)
	expect(t, status, 401)
	status, v, _ = h.call(t, "GET", "/api/health", nil, false, nil)
	expect(t, status, 200)
	if v["runtime"] != "go" {
		t.Fatal(v)
	}
}
func TestSetupSessionAndCSRF(t *testing.T) {
	h := newHarness(t)
	status, _, _ := h.call(t, "POST", "/api/admin/setup", map[string]string{"username": "admin", "password": "short"}, true, nil)
	expect(t, status, 400)
	status, _, _ = h.call(t, "POST", "/api/admin/setup", map[string]string{"username": "admin", "password": "go-test-password-123"}, true, map[string]string{"Origin": "https://evil.example"})
	expect(t, status, 403)
	h.setup(t)
	status, v, res := h.call(t, "GET", "/api/admin/session", nil, true, nil)
	expect(t, status, 200)
	if v["authenticated"] != true || v["username"] != "admin" {
		t.Fatal(v)
	}
	if res.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing security header")
	}
	status, _, _ = h.call(t, "POST", "/api/admin/setup", map[string]string{"username": "admin", "password": "go-test-password-123"}, true, nil)
	expect(t, status, 409)
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]string{"name": "test"}, true, map[string]string{"X-CSRF-Token": "invalid"})
	expect(t, status, 403)
}
func uploadImage(t *testing.T, h *harness) map[string]any {
	t.Helper()
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO/aN8sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	status, v, _ := h.call(t, "POST", "/api/admin/uploads", png, true, map[string]string{"Content-Type": "application/octet-stream", "X-File-Name": "test.png"})
	expect(t, status, 201)
	return v
}
func TestUploadPublishingAndRange(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	status, _, _ := h.call(t, "POST", "/api/admin/uploads", []byte("<svg onload=alert(1)>"), true, map[string]string{"X-File-Name": "fake.png"})
	expect(t, status, 415)
	up := uploadImage(t, h)
	path := up["url"].(string)
	status, _, _ = h.call(t, "GET", path, nil, false, nil)
	expect(t, status, 404)
	status, _, _ = h.call(t, "GET", path, nil, true, nil)
	expect(t, status, 200)
	c := contentCase(t, h, "life-forward")
	c.ID = ""
	c.Title = "Draft upload"
	c.Cover = path
	c.Gallery = []string{path}
	c.Status = "draft"
	c.Hero = false
	status, v, _ := h.call(t, "POST", "/api/admin/cases", c, true, nil)
	expect(t, status, 201)
	id := v["id"].(string)
	status, _, _ = h.call(t, "GET", "/api/public/cases/"+id, nil, false, nil)
	expect(t, status, 404)
	c.ID = id
	c.Status = "published"
	status, _, _ = h.call(t, "PUT", "/api/admin/cases/"+id, c, true, nil)
	expect(t, status, 200)
	status, v, res := h.call(t, "GET", path, nil, false, map[string]string{"Range": "bytes=0-7"})
	expect(t, status, 206)
	if len(v["bytes"].([]byte)) != 8 || res.Header.Get("Content-Range") == "" {
		t.Fatal("range failed")
	}
	status, _, _ = h.call(t, "GET", path, nil, false, map[string]string{"Range": "bytes=999999-"})
	expect(t, status, 416)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+up["id"].(string), nil, true, nil)
	expect(t, status, 409)
	c.Status = "archived"
	status, _, _ = h.call(t, "PUT", "/api/admin/cases/"+id, c, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "GET", path, nil, false, nil)
	expect(t, status, 404)
	status, _, _ = h.call(t, "DELETE", "/api/admin/cases/"+id, nil, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+up["id"].(string), nil, true, nil)
	expect(t, status, 200)
}
func TestCaseValidationHeroAndTaxonomy(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	c := contentCase(t, h, "life-forward")
	for _, mutate := range []func(*Case){func(c *Case) { c.CategoryID = "missing" }, func(c *Case) { c.Cover = "javascript:alert(1)" }, func(c *Case) { c.ExperienceURL = "javascript:alert(1)" }, func(c *Case) { c.Gallery = make([]string, 31) }} {
		v := c
		mutate(&v)
		status, _, _ := h.call(t, "POST", "/api/admin/cases", v, true, nil)
		expect(t, status, 400)
	}
	for _, path := range []string{"/api/admin/categories/content", "/api/admin/clients/icbc"} {
		status, _, _ := h.call(t, "DELETE", path, nil, true, nil)
		expect(t, status, 409)
	}
	c = contentCase(t, h, "thinkbook")
	c.Hero = true
	status, _, _ := h.call(t, "PUT", "/api/admin/cases/thinkbook", c, true, nil)
	expect(t, status, 200)
	items, _ := listRows[Case](h.app.Store.DB, "cases")
	count := 0
	for _, c := range items {
		if c.Hero {
			count++
		}
	}
	if count != 1 {
		t.Fatal("multiple hero cases")
	}
	status, v, _ := h.call(t, "POST", "/api/admin/clients", Client{Name: "New client", Order: 20}, true, nil)
	expect(t, status, 201)
	id := v["id"].(string)
	status, _, _ = h.call(t, "PUT", "/api/admin/clients/"+id, Client{Name: "Edited client", Order: 30}, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/clients/"+id, nil, true, nil)
	expect(t, status, 200)
}
func TestSettingsPlatformsAndExport(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	status, _, _ := h.call(t, "PUT", "/api/admin/company", map[string]any{"slogan": "Go persisted", "homeFeaturedLimit": 3}, true, nil)
	expect(t, status, 200)
	for _, v := range []map[string]any{{"homeFeaturedLimit": 0}, {"email": "invalid"}, {"name": nil}, {"qualificationImages": []string{"https://evil.example/image.jpg"}}} {
		status, _, _ = h.call(t, "PUT", "/api/admin/company", v, true, nil)
		expect(t, status, 400)
	}
	setting, _ := getRow[platformSetting](h.app.Store.DB, "settings", "platforms")
	setting.Items[0].URL = "https://example.com/geo"
	status, _, _ = h.call(t, "PUT", "/api/admin/platforms", setting, true, nil)
	expect(t, status, 200)
	setting.Items[0].URL = "javascript:alert(1)"
	status, _, _ = h.call(t, "PUT", "/api/admin/platforms", setting, true, nil)
	expect(t, status, 400)
	setting.Items = []Platform{setting.Items[1], setting.Items[1]}
	status, _, _ = h.call(t, "PUT", "/api/admin/platforms", setting, true, nil)
	expect(t, status, 400)
	status, v, _ := h.call(t, "GET", "/api/admin/export", nil, true, nil)
	expect(t, status, 200)
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "password") || strings.Contains(string(raw), "csrf") {
		t.Fatal("export exposes auth data")
	}
}
func TestPasswordRotationAndLogout(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	u, _ := url.Parse(h.server.URL)
	oldCookie := h.client.Jar.Cookies(u)[0].String()
	status, _, _ := h.call(t, "POST", "/api/admin/password", map[string]string{"currentPassword": "wrong", "password": "new-go-password-456"}, true, nil)
	expect(t, status, 400)
	status, v, _ := h.call(t, "POST", "/api/admin/password", map[string]string{"currentPassword": "go-test-password-123", "password": "new-go-password-456"}, true, nil)
	expect(t, status, 200)
	h.csrf = v["csrf"].(string)
	status, _, _ = h.call(t, "GET", "/api/admin/config", nil, false, map[string]string{"Cookie": oldCookie})
	expect(t, status, 401)
	status, _, _ = h.call(t, "POST", "/api/admin/logout", map[string]string{}, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "GET", "/api/admin/config", nil, true, nil)
	expect(t, status, 401)
	status, _, _ = h.call(t, "POST", "/api/admin/login", map[string]string{"username": "admin", "password": "go-test-password-123"}, true, nil)
	expect(t, status, 401)
	status, v, res := h.call(t, "POST", "/api/admin/login", map[string]string{"username": "admin", "password": "new-go-password-456"}, true, nil)
	expect(t, status, 200)
	h.csrf = v["csrf"].(string)
	if !strings.Contains(res.Header.Get("Set-Cookie"), "HttpOnly") || !strings.Contains(res.Header.Get("Set-Cookie"), "SameSite=Strict") {
		t.Fatal("unsafe cookie")
	}
}
func TestExistingNodePasswordAndSession(t *testing.T) {
	h := newHarness(t)
	var fixture struct{ Password, Hash, Token, Digest, CSRF string }
	raw, err := os.ReadFile("testdata/node-auth.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(fixture.Password, fixture.Hash) {
		t.Fatal("Node scrypt hash is incompatible")
	}
	if verifyPassword("wrong", fixture.Hash) {
		t.Fatal("incorrect password accepted")
	}
	err = h.app.Store.DB.Create(&admin{ID: 1, Username: "legacyadmin", Password: fixture.Hash}).Error
	if err != nil {
		t.Fatal(err)
	}
	err = h.app.Store.DB.Create(&session{Hash: fixture.Digest, CSRF: fixture.CSRF, Expires: time.Now().Add(time.Hour).UnixMilli()}).Error
	if err != nil {
		t.Fatal(err)
	}
	status, v, _ := h.call(t, "GET", "/api/admin/session", nil, false, map[string]string{"Cookie": "ytl_session=" + fixture.Token})
	expect(t, status, 200)
	if v["authenticated"] != true || v["csrf"] != fixture.CSRF || v["username"] != "legacyadmin" {
		t.Fatal("old session not preserved")
	}
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]string{"slogan": "Legacy session still works"}, false, map[string]string{"Cookie": "ytl_session=" + fixture.Token, "X-CSRF-Token": fixture.CSRF})
	expect(t, status, 200)
	status, _, _ = h.call(t, "POST", "/api/admin/login", map[string]string{"username": "legacyadmin", "password": fixture.Password}, true, nil)
	expect(t, status, 200)
}
func TestRestartPreservesDataAndSession(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	status, _, _ := h.call(t, "PUT", "/api/admin/company", map[string]string{"slogan": "Persistence checked"}, true, nil)
	expect(t, status, 200)
	h.server.Close()
	h.app.Close()
	a, err := New(Config{Root: h.root, DataDir: h.dataDir})
	if err != nil {
		t.Fatal(err)
	}
	h.app = a
	h.server = httptest.NewServer(a)
	status, v, _ := h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	if v["company"].(map[string]any)["slogan"] != "Persistence checked" {
		t.Fatal("content lost")
	}
	status, _, _ = h.call(t, "GET", "/api/admin/config", nil, true, nil)
	expect(t, status, 200)
}
func TestConcurrentHeroUpdates(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	left, right := contentCase(t, h, "life-forward"), contentCase(t, h, "thinkbook")
	left.Hero = true
	right.Hero = true
	var wg sync.WaitGroup
	var failed atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := left
			if i%2 == 1 {
				c = right
			}
			raw, _ := json.Marshal(c)
			req, _ := http.NewRequest("PUT", h.server.URL+"/api/admin/cases/"+c.ID, bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", h.server.URL)
			req.Header.Set("X-CSRF-Token", h.csrf)
			res, err := h.client.Do(req)
			if err != nil {
				failed.Add(1)
				return
			}
			defer res.Body.Close()
			io.Copy(io.Discard, res.Body)
			if res.StatusCode != 200 {
				failed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if failed.Load() != 0 {
		t.Fatalf("%d concurrent updates failed", failed.Load())
	}
	items, err := listRows[Case](h.app.Store.DB, "cases")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, c := range items {
		if c.Hero {
			count++
		}
	}
	if count != 1 {
		t.Fatal("concurrency broke single hero")
	}
}
func TestBackupRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	up := uploadImage(t, h)
	c := contentCase(t, h, "life-forward")
	c.Cover = up["url"].(string)
	status, _, _ := h.call(t, "PUT", "/api/admin/cases/life-forward", c, true, nil)
	expect(t, status, 200)
	dest := filepath.Join(t.TempDir(), "backup")
	if err := h.app.Backup(dest); err != nil {
		t.Fatal(err)
	}
	restored, err := New(Config{Root: h.root, DataDir: dest})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	stored, err := getRow[Case](restored.Store.DB, "cases", "life-forward")
	if err != nil || stored.Cover != c.Cover {
		t.Fatal("backup content incorrect")
	}
	if _, err = os.Stat(filepath.Join(dest, "uploads", filepath.Base(c.Cover))); err != nil {
		t.Fatal("backup file missing")
	}
	admin, err := restored.currentAdmin()
	if err != nil || !verifyPassword("go-test-password-123", admin.Password) {
		t.Fatal("backup admin incompatible")
	}
	if err = h.app.Backup(dest); err == nil {
		t.Fatal("existing backup overwritten")
	}
	if err = h.app.Backup(filepath.Join(h.dataDir, "nested")); err == nil {
		t.Fatal("unsafe backup accepted")
	}
}
func TestLimitsOriginAndProduction(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	h.app.cfg.MaxUpload = 512
	status, _, _ := h.call(t, "POST", "/api/admin/uploads", make([]byte, 1024), true, map[string]string{"X-File-Name": "large.png"})
	expect(t, status, 413)
	req, _ := http.NewRequest("GET", h.server.URL+"/api/public/config", nil)
	req.Host = "evil.example"
	res, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	expect(t, res.StatusCode, 403)
	if _, err = New(Config{Root: h.root, DataDir: t.TempDir(), Production: true}); err == nil {
		t.Fatal("production allowed without HTTPS origin")
	}
	for i := 0; i < 10; i++ {
		status, _, _ = h.call(t, "POST", "/api/admin/login", map[string]string{"username": "admin", "password": "invalid"}, false, nil)
		expect(t, status, 401)
	}
	status, _, _ = h.call(t, "POST", "/api/admin/login", map[string]string{"username": "admin", "password": "invalid"}, false, nil)
	expect(t, status, 429)
}
