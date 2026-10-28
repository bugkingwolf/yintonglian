package app

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"io"
	"io/fs"
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
func (a *Server) mediaValue(ctx context.Context, value, kind string) (string, error) {
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
				if _, e := a.Files.Stat(ctx, file.Filename); e == nil {
					return v, nil
				} else if !errors.Is(e, fs.ErrNotExist) {
					return "", e
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
	setAuditTarget(r, "", original)
	id, err := newID()
	if err != nil {
		return err
	}
	// A private temporary spool validates bytes before they enter persistent storage.
	f, err := os.CreateTemp("", "yintonglian-upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
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
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	filename := id + "." + kind.Extension
	if err = a.Files.Put(r.Context(), filename, f); err != nil {
		return err
	}
	err = a.Store.DB.Create(&Upload{ID: id, Filename: filename, MIME: kind.MIME, Size: size, OriginalName: original, CreatedAt: time.Now().UnixMilli()}).Error
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
		defer cancel()
		if cleanupErr := a.Files.Delete(cleanup, filename); cleanupErr != nil {
			return errors.Join(err, cleanupErr)
		}
		return err
	}
	setAuditTarget(r, id, original)
	return respond(w, 201, map[string]any{"id": id, "url": "/media/" + filename, "mime": kind.MIME, "size": size, "name": original})
}
func mediaReferenced(c Case, url string) bool {
	return c.Cover == url || c.Video == url || slices.Contains(c.Gallery, url)
}
func (a *Server) mediaPublished(url string) (bool, error) {
	var count int64
	if err := a.Store.DB.Table("cases").Where("status = ?", "published").Where("json_extract(payload,'$.cover') = ? OR json_extract(payload,'$.video') = ? OR EXISTS (SELECT 1 FROM json_each(cases.payload,'$.gallery') WHERE value = ?)", url, url, url).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	company, err := getRow[Company](a.Store.DB, "settings", "company")
	if err != nil {
		return false, err
	}
	if slices.Contains(company.QualificationImages, url) {
		return true, nil
	}
	for _, m := range company.TeamMembers {
		if m.Enabled && (m.Photo == url || m.WechatQR == url) {
			return true, nil
		}
	}
	return false, nil
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
	if delivery, ok := a.Files.(SignedMediaStorage); ok {
		target, err := delivery.SignedURL(r.Context(), file.Filename, r.Method, 5*time.Minute)
		if err != nil {
			return err
		}
		parsed, err := url.Parse(target)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
			return errors.New("storage returned invalid HTTPS delivery URL")
		}
		w.Header().Set("Cache-Control", "private, no-store")
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
		return nil
	}
	object, err := a.Files.Open(r.Context(), file.Filename)
	if errors.Is(err, fs.ErrNotExist) {
		return bad(404, "文件不存在")
	}
	if err != nil {
		return err
	}
	defer object.Close()
	w.Header().Set("Content-Type", file.MIME)
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, file.Filename, object.Modified, object)
	return nil
}
func (a *Server) deleteUpload(w http.ResponseWriter, r *http.Request) error {
	var x Upload
	err := a.Store.DB.Where("id = ?", r.PathValue("id")).Take(&x).Error
	if err != nil {
		return recordError(err, "文件不存在")
	}
	url := "/media/" + x.Filename
	var count int64
	if err = a.Store.DB.Table("cases").Where("json_extract(payload,'$.cover') = ? OR json_extract(payload,'$.video') = ? OR EXISTS (SELECT 1 FROM json_each(cases.payload,'$.gallery') WHERE value = ?)", url, url, url).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return bad(409, "该文件被内容引用，请先移除引用")
	}
	company, err := getRow[Company](a.Store.DB, "settings", "company")
	if err != nil {
		return err
	}
	if slices.Contains(company.QualificationImages, url) {
		return bad(409, "该文件被内容引用，请先移除引用")
	}
	for _, m := range company.TeamMembers {
		if m.Photo == url || m.WechatQR == url {
			return bad(409, "该文件被团队成员引用，请先移除引用")
		}
	}
	if err = a.Files.Delete(r.Context(), x.Filename); err != nil {
		return err
	}
	if err = a.Store.DB.Where("id = ?", x.ID).Delete(&Upload{}).Error; err != nil {
		return err
	}
	return respond(w, 200, map[string]bool{"ok": true})
}
