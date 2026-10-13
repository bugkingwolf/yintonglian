package app

import (
	"errors"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type mediaType struct{ Extension, MIME, Kind string }

func detectMedia(header []byte) (mediaType, bool) {
	s := string(header)
	if len(header) >= 8 && s[:8] == "\x89PNG\r\n\x1a\n" {
		return mediaType{"png", "image/png", "image"}, true
	}
	if len(header) >= 3 && s[:3] == "\xff\xd8\xff" {
		return mediaType{"jpg", "image/jpeg", "image"}, true
	}
	if len(header) >= 12 && s[:4] == "RIFF" && s[8:12] == "WEBP" {
		return mediaType{"webp", "image/webp", "image"}, true
	}
	if len(header) >= 12 && s[4:8] == "ftyp" {
		return mediaType{"mp4", "video/mp4", "video"}, true
	}
	if len(header) >= 4 && s[:4] == "\x1a\x45\xdf\xa3" {
		return mediaType{"webm", "video/webm", "video"}, true
	}
	return mediaType{}, false
}
func (a *Server) mediaValue(value, kind string) (string, error) {
	v, err := clean(value, 512)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", nil
	}
	if kind == "image" && assetPattern.MatchString(v) && a.assetExists(v) {
		return v, nil
	}
	if strings.HasPrefix(v, "/media/") {
		name := strings.TrimPrefix(v, "/media/")
		if uploadPattern.MatchString(name) {
			file, err := a.Store.Upload(name)
			if err == nil && strings.HasPrefix(file.MIME, kind+"/") {
				if info, e := os.Stat(filepath.Join(a.Store.DataDir, "uploads", file.Filename)); e == nil && info.Mode().IsRegular() {
					return v, nil
				}
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return "", err
			}
		}
	}
	return "", bad(400, "请选择已上传的图片或视频")
}
func (a *Server) upload(w http.ResponseWriter, r *http.Request) error {
	if r.ContentLength > a.cfg.MaxUpload {
		return bad(413, "文件超过上传大小限制")
	}
	original, err := url.PathUnescape(r.Header.Get("X-File-Name"))
	if err != nil {
		return bad(400, "文件名不正确")
	}
	original = filepath.Base(strings.ReplaceAll(original, "\\", "/"))
	if original, err = clean(original, 200); err != nil {
		return err
	}
	if original == "" || original == "." {
		return bad(400, "请选择文件")
	}
	id, err := newID()
	if err != nil {
		return err
	}
	dir := filepath.Join(a.Store.DataDir, "uploads")
	tmp := filepath.Join(dir, id+".tmp")
	f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	defer f.Close()
	body := http.MaxBytesReader(w, r.Body, a.cfg.MaxUpload)
	defer body.Close()
	size, err := io.Copy(f, body)
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return bad(413, "文件超过上传大小限制")
		}
		return err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	header := make([]byte, 64)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return err
	}
	kind, ok := detectMedia(header[:n])
	if !ok {
		return bad(415, "支持JPG、PNG、WebP图片或MP4、WebM视频，请检查文件内容")
	}
	if kind.Kind == "image" && size > 20*1024*1024 {
		return bad(413, "图片不能超过20MB")
	}
	if err = f.Close(); err != nil {
		return err
	}
	filename := id + "." + kind.Extension
	dest := filepath.Join(dir, filename)
	if err = os.Rename(tmp, dest); err != nil {
		return err
	}
	err = a.Store.DB.Create(&Upload{ID: id, Filename: filename, MIME: kind.MIME, Size: size, OriginalName: original, CreatedAt: time.Now().UnixMilli()}).Error
	if err != nil {
		os.Remove(dest)
		return err
	}
	return respond(w, 201, map[string]any{"id": id, "url": "/media/" + filename, "mime": kind.MIME, "size": size, "name": original})
}
func mediaReferenced(c Case, url string) bool {
	return c.Cover == url || c.Video == url || slices.Contains(c.Gallery, url)
}
func (a *Server) mediaPublished(url string) (bool, error) {
	items, err := a.published()
	if err != nil {
		return false, err
	}
	for _, c := range items {
		if mediaReferenced(c, url) {
			return true, nil
		}
	}
	company, err := getRow[Company](a.Store.DB, "settings", "company")
	if err != nil {
		return false, err
	}
	return slices.Contains(company.QualificationImages, url), nil
}
func (a *Server) media(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("filename")
	if !uploadPattern.MatchString(name) {
		return bad(404, "文件不存在")
	}
	file, err := a.Store.Upload(name)
	if err != nil {
		return recordError(err, "文件不存在")
	}
	s, err := a.findSession(r)
	if err != nil {
		return err
	}
	if s == nil {
		allowed, err := a.mediaPublished("/media/" + name)
		if err != nil {
			return err
		}
		if !allowed {
			return bad(404, "文件不存在")
		}
	}
	return serveFile(w, r, filepath.Join(a.Store.DataDir, "uploads", file.Filename), file.MIME, "private, no-store")
}
func (a *Server) deleteUpload(w http.ResponseWriter, r *http.Request) error {
	var x Upload
	err := a.Store.DB.Where("id = ?", r.PathValue("id")).Take(&x).Error
	if err != nil {
		return recordError(err, "文件不存在")
	}
	url := "/media/" + x.Filename
	items, err := listRows[Case](a.Store.DB, "cases")
	if err != nil {
		return err
	}
	for _, c := range items {
		if mediaReferenced(c, url) {
			return bad(409, "该文件被内容引用，请先移除引用")
		}
	}
	company, err := getRow[Company](a.Store.DB, "settings", "company")
	if err != nil {
		return err
	}
	if slices.Contains(company.QualificationImages, url) {
		return bad(409, "该文件被内容引用，请先移除引用")
	}
	if err = os.Remove(filepath.Join(a.Store.DataDir, "uploads", x.Filename)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = a.Store.DB.Where("id = ?", x.ID).Delete(&Upload{}).Error; err != nil {
		return err
	}
	return respond(w, 200, map[string]bool{"ok": true})
}
