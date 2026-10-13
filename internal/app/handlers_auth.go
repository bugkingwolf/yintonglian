package app

import (
	"errors"
	"gorm.io/gorm"
	"net"
	"net/http"
	"time"
)

func (a *Server) currentAdmin() (admin, error) {
	var x admin
	err := a.Store.DB.Where("id = ?", 1).Take(&x).Error
	return x, err
}
func (a *Server) findSession(r *http.Request) (*session, error) {
	c, err := r.Cookie("ytl_session")
	if errors.Is(err, http.ErrNoCookie) {
		return nil, nil
	}
	if err != nil {
		return nil, nil
	}
	var s session
	err = a.Store.DB.Where("hash = ? AND expires > ?", tokenHash(c.Value), time.Now().UnixMilli()).Take(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
func (a *Server) sameOrigin(r *http.Request) error {
	origin := a.cfg.Origin
	if origin == "" {
		origin = "http://" + r.Host
	}
	if r.Header.Get("Origin") != origin {
		return bad(403, "来源验证失败，请刷新页面重试")
	}
	return nil
}
func (a *Server) authorize(r *http.Request) (*session, error) {
	s, err := a.findSession(r)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, bad(401, "请先登录管理后台")
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if err = a.sameOrigin(r); err != nil {
			return nil, err
		}
		if r.Header.Get("X-CSRF-Token") != s.CSRF {
			return nil, bad(403, "安全验证失败，请重新登录")
		}
	}
	return s, nil
}
func (a *Server) issueSession(w http.ResponseWriter) (string, error) {
	value, err := randomToken()
	if err != nil {
		return "", err
	}
	csrf, err := randomToken()
	if err != nil {
		return "", err
	}
	tx := a.Store.DB.Begin()
	if err = tx.Error; err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = tx.Where("expires < ?", time.Now().UnixMilli()).Delete(&session{}).Error; err != nil {
		return "", err
	}
	if err = tx.Create(&session{Hash: tokenHash(value), CSRF: csrf, Expires: time.Now().Add(8 * time.Hour).UnixMilli()}).Error; err != nil {
		return "", err
	}
	if err = tx.Commit().Error; err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{Name: "ytl_session", Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: a.cfg.Production, MaxAge: 28800})
	return csrf, nil
}
func (a *Server) sessionInfo(w http.ResponseWriter, r *http.Request) error {
	s, err := a.findSession(r)
	if err != nil {
		return err
	}
	user, err := a.currentAdmin()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var csrf, username any
	if s != nil {
		csrf = s.CSRF
		username = user.Username
	}
	return respond(w, 200, map[string]any{"authenticated": s != nil, "setupRequired": errors.Is(err, gorm.ErrRecordNotFound), "csrf": csrf, "username": username})
}
func (a *Server) CreateAdmin(username, password string) error {
	if !usernamePattern.MatchString(username) {
		return bad(400, "账号需为3至60位英文或数字")
	}
	if !validPassword(password) {
		return bad(400, "密码需为12至128个字符")
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	_, err := a.currentAdmin()
	if err == nil {
		return bad(409, "管理员已设置")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	err = a.Store.DB.Create(&admin{ID: 1, Username: username, Password: hash}).Error
	return err
}
func (a *Server) setup(w http.ResponseWriter, r *http.Request) error {
	if err := a.sameOrigin(r); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if a.cfg.Production || ip == nil || !ip.IsLoopback() {
		return bad(403, "请在服务器命令行初始化管理员")
	}
	var v struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err = decode(w, r, &v); err != nil {
		return err
	}
	if v.Username == "" {
		v.Username = "admin"
	}
	v.Username, err = clean(v.Username, 60)
	if err != nil {
		return err
	}
	if err = a.CreateAdmin(v.Username, v.Password); err != nil {
		return err
	}
	csrf, err := a.issueSession(w)
	if err != nil {
		return err
	}
	return respond(w, 201, map[string]string{"csrf": csrf, "username": v.Username})
}
func (a *Server) login(w http.ResponseWriter, r *http.Request) error {
	if err := a.sameOrigin(r); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	a.limitMu.Lock()
	now := time.Now()
	for key, item := range a.attempts {
		if now.After(item.Until) {
			delete(a.attempts, key)
		}
	}
	item := a.attempts[host]
	if item.Until.IsZero() {
		item.Until = now.Add(15 * time.Minute)
	}
	if item.Count >= 10 {
		a.limitMu.Unlock()
		return bad(429, "尝试次数过多，请稍后再试")
	}
	item.Count++
	a.attempts[host] = item
	a.limitMu.Unlock()
	var v struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err = decode(w, r, &v); err != nil {
		return err
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	user, err := a.currentAdmin()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return bad(409, "请先设置管理员")
	}
	if err != nil {
		return err
	}
	if v.Username != user.Username || !verifyPassword(v.Password, user.Password) {
		return bad(401, "账号或密码不正确")
	}
	a.limitMu.Lock()
	delete(a.attempts, host)
	a.limitMu.Unlock()
	csrf, err := a.issueSession(w)
	if err != nil {
		return err
	}
	return respond(w, 200, map[string]string{"csrf": csrf, "username": user.Username})
}
func (a *Server) logout(w http.ResponseWriter, r *http.Request) error {
	s := r.Context().Value(sessionKey{}).(*session)
	if err := a.Store.DB.Where("hash = ?", s.Hash).Delete(&session{}).Error; err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "ytl_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: a.cfg.Production, MaxAge: -1})
	return respond(w, 200, map[string]bool{"ok": true})
}
func (a *Server) changePassword(w http.ResponseWriter, r *http.Request) error {
	var v struct {
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
	}
	if err := decode(w, r, &v); err != nil {
		return err
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	user, err := a.currentAdmin()
	if err != nil {
		return err
	}
	if !verifyPassword(v.CurrentPassword, user.Password) {
		return bad(400, "当前密码不正确")
	}
	if !validPassword(v.Password) {
		return bad(400, "新密码需为12至128个字符")
	}
	hash, err := HashPassword(v.Password)
	if err != nil {
		return err
	}
	tx := a.Store.DB.Begin()
	if err = tx.Error; err != nil {
		return err
	}
	defer tx.Rollback()
	if err = tx.Model(&admin{}).Where("id = ?", 1).Update("password", hash).Error; err != nil {
		return err
	}
	if err = tx.Where("1 = 1").Delete(&session{}).Error; err != nil {
		return err
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	csrf, err := a.issueSession(w)
	if err != nil {
		return err
	}
	return respond(w, 200, map[string]any{"ok": true, "csrf": csrf})
}
