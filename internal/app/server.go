package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Root       string
	DataDir    string
	Origin     string
	Production bool
	MaxUpload  int64
}
type Server struct {
	Store    *Store
	cfg      Config
	mux      *http.ServeMux
	writeMu  sync.Mutex
	authMu   sync.Mutex
	limitMu  sync.Mutex
	attempts map[string]attempt
}
type attempt struct {
	Count int
	Until time.Time
}
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string         { return e.Message }
func bad(status int, message string) error { return &HTTPError{status, message} }

type handler func(http.ResponseWriter, *http.Request) error
type sessionKey struct{}

func New(cfg Config) (*Server, error) {
	var err error
	if cfg.Root == "" {
		cfg.Root = "."
	}
	if cfg.Root, err = filepath.Abs(cfg.Root); err != nil {
		return nil, err
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(cfg.Root, ".data")
	}
	if cfg.MaxUpload == 0 {
		cfg.MaxUpload = 300 * 1024 * 1024
	}
	if cfg.MaxUpload < 1 {
		return nil, fmt.Errorf("invalid upload limit")
	}
	if cfg.Origin != "" {
		u, e := url.Parse(cfg.Origin)
		if e != nil || u.Hostname() == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("APP_ORIGIN must be an HTTP(S) origin without path")
		}
		cfg.Origin = strings.TrimRight(cfg.Origin, "/")
	}
	if cfg.Production && !strings.HasPrefix(cfg.Origin, "https://") {
		return nil, fmt.Errorf("production requires HTTPS APP_ORIGIN")
	}
	store, err := OpenStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	a := &Server{Store: store, cfg: cfg, mux: http.NewServeMux(), attempts: make(map[string]attempt)}
	a.routes()
	return a, nil
}
func (a *Server) Close() error { return a.Store.Close() }
func (a *Server) routes() {
	a.route("GET /api/health", false, func(w http.ResponseWriter, r *http.Request) error {
		return respond(w, 200, map[string]any{"ok": true, "runtime": "go", "version": "0.2.0"})
	})
	a.route("GET /api/public/config", false, a.publicConfig)
	a.route("GET /api/public/cases", false, a.publicCases)
	a.route("GET /api/public/cases/{id}", false, a.publicCase)
	a.route("GET /api/admin/session", false, a.sessionInfo)
	a.route("POST /api/admin/setup", false, a.setup)
	a.route("POST /api/admin/login", false, a.login)
	a.route("POST /api/admin/logout", true, a.logout)
	a.route("POST /api/admin/password", true, a.changePassword)
	a.route("GET /api/admin/content", true, a.adminContent)
	a.route("POST /api/admin/cases", true, a.saveCase)
	a.route("PUT /api/admin/cases/{id}", true, a.saveCase)
	a.route("DELETE /api/admin/cases/{id}", true, a.deleteCase)
	for _, table := range []string{"categories", "clients"} {
		a.route("POST /api/admin/"+table, true, a.saveTaxonomy(table))
		a.route("PUT /api/admin/"+table+"/{id}", true, a.saveTaxonomy(table))
		a.route("DELETE /api/admin/"+table+"/{id}", true, a.deleteTaxonomy(table))
	}
	a.route("PUT /api/admin/company", true, a.saveCompany)
	a.route("PUT /api/admin/platforms", true, a.savePlatforms)
	a.route("GET /api/admin/export", true, a.exportContent)
	a.route("POST /api/admin/uploads", true, a.upload)
	a.route("DELETE /api/admin/uploads/{id}", true, a.deleteUpload)
	a.route("GET /media/{filename}", false, a.media)
	a.route("GET /", false, a.static)
}
func (a *Server) route(pattern string, protected bool, h handler) {
	a.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if protected {
			if r.Method != "GET" && r.Method != "HEAD" {
				a.writeMu.Lock()
				defer a.writeMu.Unlock()
			}
			s, err := a.authorize(r)
			if err != nil {
				writeError(w, err)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, s))
		}
		if err := h(w, r); err != nil {
			writeError(w, err)
		}
	})
}
func (a *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; font-src 'self'; frame-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	if a.cfg.Production {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if a.cfg.Origin != "" {
		u, _ := url.Parse(a.cfg.Origin)
		if !strings.EqualFold(host, u.Hostname()) {
			writeError(w, bad(403, "Host not allowed"))
			return
		}
	} else if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		writeError(w, bad(403, "Host not allowed"))
		return
	}
	a.mux.ServeHTTP(w, r)
}
func respond(w http.ResponseWriter, status int, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(append(raw, '\n'))
	return err
}
func writeError(w http.ResponseWriter, err error) {
	var e *HTTPError
	status, message := 500, "服务暂时不可用，请稍后重试"
	if errors.As(err, &e) {
		status, message = e.Status, e.Message
	} else {
		log.Printf("request failed: %v", err)
	}
	_ = respond(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, value any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return bad(415, "请求需要JSON内容")
	}
	body := http.MaxBytesReader(w, r.Body, 1024*1024)
	defer body.Close()
	d := json.NewDecoder(body)
	if err := d.Decode(value); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return bad(413, "请求内容过大")
		}
		return bad(400, "JSON格式不正确")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return bad(400, "JSON格式不正确")
	}
	return nil
}
func clean(value string, max int) (string, error) {
	if textLength(value) > max {
		return "", bad(400, "文字内容超过长度限制")
	}
	return strings.TrimSpace(value), nil
}
func cleanList(items []string, maxItems int) ([]string, error) {
	if len(items) > maxItems {
		return nil, bad(400, "列表条目超过数量限制")
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item, err := clean(item, 120)
		if err != nil {
			return nil, err
		}
		if item != "" {
			out = append(out, item)
		}
	}
	return out, nil
}
func validOrder(value int) error {
	if value < -100000 || value > 100000 {
		return bad(400, "排序需为有效整数")
	}
	return nil
}
func recordError(err error, message string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return bad(404, message)
	}
	return err
}
func (a *Server) assetExists(path string) bool {
	info, err := os.Stat(filepath.Join(a.cfg.Root, "public", strings.TrimPrefix(path, "/")))
	return err == nil && info.Mode().IsRegular()
}
