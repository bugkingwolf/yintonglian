package app

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func (a *Server) saveTaxonomy(table string) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		var err error
		status := 200
		if r.Method == "POST" {
			status = 201
			id, err = newID()
			if err != nil {
				return err
			}
		} else {
			if _, err = getRow[json.RawMessage](a.Store.DB, table, id); err != nil {
				return recordError(err, "内容不存在")
			}
		}
		var value any
		if table == "categories" {
			var c Category
			if err = decode(w, r, &c); err != nil {
				return err
			}
			c.ID = id
			if c.Name, err = clean(c.Name, 80); err != nil {
				return err
			}
			if c.Name == "" {
				return bad(400, "请填写名称")
			}
			if c.Subtitle, err = clean(c.Subtitle, 200); err != nil {
				return err
			}
			if c.Description, err = clean(c.Description, 2000); err != nil {
				return err
			}
			if c.Services, err = cleanList(c.Services, 30); err != nil {
				return err
			}
			if err = validOrder(c.Order); err != nil {
				return err
			}
			value = c
		} else {
			var c Client
			if err = decode(w, r, &c); err != nil {
				return err
			}
			c.ID = id
			if c.Name, err = clean(c.Name, 80); err != nil {
				return err
			}
			if c.Name == "" {
				return bad(400, "请填写名称")
			}
			if err = validOrder(c.Order); err != nil {
				return err
			}
			value = c
		}
		if err = putRow(a.Store.DB, table, value); err != nil {
			return err
		}
		return respond(w, status, value)
	}
}
func (a *Server) deleteTaxonomy(table string) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := deleteRow(a.Store.DB, table, r.PathValue("id")); err != nil {
			if strings.Contains(err.Error(), "FOREIGN KEY") {
				return bad(409, "该分类或客户已有案例，请先调整案例归属")
			}
			return recordError(err, "内容不存在")
		}
		return respond(w, 200, map[string]bool{"ok": true})
	}
}

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func (a *Server) saveCompany(w http.ResponseWriter, r *http.Request) error {
	var changes map[string]json.RawMessage
	if err := decode(w, r, &changes); err != nil {
		return err
	}
	if changes == nil {
		return bad(400, "JSON内容不正确")
	}
	c, err := getRow[Company](a.Store.DB, "settings", "company")
	if err != nil {
		return err
	}
	fields := map[string]*string{"name": &c.Name, "english": &c.English, "legalName": &c.LegalName, "headline": &c.Headline, "slogan": &c.Slogan, "introduction": &c.Introduction, "vision": &c.Vision, "philosophy": &c.Philosophy, "team": &c.Team, "qualifications": &c.Qualifications, "phone": &c.Phone, "email": &c.Email, "address": &c.Address, "contactNote": &c.ContactNote, "founded": &c.Founded}
	for key, ptr := range fields {
		raw, ok := changes[key]
		if !ok {
			continue
		}
		var v string
		if string(raw) == "null" || json.Unmarshal(raw, &v) != nil {
			return bad(400, "文字内容格式不正确")
		}
		max := 500
		if key == "introduction" || key == "vision" || key == "philosophy" || key == "team" || key == "qualifications" {
			max = 10000
		}
		if *ptr, err = clean(v, max); err != nil {
			return err
		}
	}
	if c.Name == "" {
		return bad(400, "公司名称不能为空")
	}
	if c.Email != "" && !emailPattern.MatchString(c.Email) {
		return bad(400, "邮箱格式不正确")
	}
	if raw, ok := changes["qualificationImages"]; ok {
		var images []string
		if string(raw) == "null" || json.Unmarshal(raw, &images) != nil || len(images) > 20 {
			return bad(400, "最多上传20张资质图片")
		}
		c.QualificationImages = make([]string, 0, len(images))
		for _, src := range images {
			v, err := a.mediaValue(src, "image")
			if err != nil {
				return err
			}
			if v == "" {
				return bad(400, "请选择有效资质图片")
			}
			c.QualificationImages = append(c.QualificationImages, v)
		}
	}
	if raw, ok := changes["homeFeaturedLimit"]; ok {
		if json.Unmarshal(raw, &c.HomeFeaturedLimit) != nil || c.HomeFeaturedLimit < 1 || c.HomeFeaturedLimit > 12 {
			return bad(400, "精选数量需为1至12")
		}
	}
	if err = putRow(a.Store.DB, "settings", c); err != nil {
		return err
	}
	return respond(w, 200, c)
}
func (a *Server) savePlatforms(w http.ResponseWriter, r *http.Request) error {
	var setting platformSetting
	if err := decode(w, r, &setting); err != nil {
		return err
	}
	if len(setting.Items) != 2 {
		return bad(400, "需配置GEO与漫剧两个入口")
	}
	seen := make(map[string]bool)
	for i := range setting.Items {
		p := &setting.Items[i]
		if (p.ID != "geo" && p.ID != "comic") || seen[p.ID] {
			return bad(400, "需分别配置GEO与漫剧入口")
		}
		seen[p.ID] = true
		var err error
		if p.Name, err = clean(p.Name, 100); err != nil {
			return err
		}
		if p.Name == "" {
			return bad(400, "请填写平台名称")
		}
		if p.Subtitle, err = clean(p.Subtitle, 200); err != nil {
			return err
		}
		if p.Description, err = clean(p.Description, 2000); err != nil {
			return err
		}
		if p.URL, err = safeLink(p.URL); err != nil {
			return bad(400, err.Error())
		}
		if p.MobileURL, err = safeLink(p.MobileURL); err != nil {
			return bad(400, err.Error())
		}
		if err = validOrder(p.Order); err != nil {
			return err
		}
	}
	setting.ID = "platforms"
	if err := putRow(a.Store.DB, "settings", setting); err != nil {
		return err
	}
	return respond(w, 200, map[string]any{"items": setting.Items})
}
func (a *Server) exportContent(w http.ResponseWriter, r *http.Request) error {
	d, err := a.Store.Snapshot()
	if err != nil {
		return err
	}
	return respond(w, 200, map[string]any{"version": 1, "exportedAt": time.Now().UTC().Format(time.RFC3339Nano), "cases": d.Cases, "categories": d.Categories, "clients": d.Clients, "company": d.Company, "platforms": d.Platforms})
}
