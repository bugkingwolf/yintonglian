package app

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var staticPattern = regexp.MustCompile(`^/(assets/[a-z0-9-]+\.jpg|admin/(admin\.css|admin\.js)|site\.css|site\.js)$`)

func (a *Server) static(w http.ResponseWriter, r *http.Request) error {
	path := r.URL.Path
	filename := ""
	switch {
	case path == "/":
		filename = "index.html"
	case path == "/admin" || path == "/admin/":
		filename = "admin/index.html"
	case staticPattern.MatchString(path):
		filename = strings.TrimPrefix(path, "/")
	case !strings.HasPrefix(path, "/api/") && filepath.Ext(path) == "":
		filename = "index.html"
	default:
		return bad(404, "页面不存在")
	}
	mime := map[string]string{".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".jpg": "image/jpeg"}[filepath.Ext(filename)]
	cache := "no-cache"
	if strings.HasPrefix(filename, "assets/") {
		cache = "public, max-age=86400"
	}
	return serveFile(w, r, filepath.Join(a.cfg.Root, "public", filename), mime, cache)
}
func serveFile(w http.ResponseWriter, r *http.Request, path, mime, cache string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return bad(404, "文件不存在")
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return bad(404, "文件不存在")
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", cache)
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	return nil
}
