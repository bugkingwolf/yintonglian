package app

import (
	"encoding/json"
	"math"
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
			setAuditTarget(r, id, c.Name)
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
			setAuditTarget(r, id, c.Name)
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
	previousMembers := c.TeamMembers
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
	for key, ptr := range map[string]**float64{"latitude": &c.Latitude, "longitude": &c.Longitude} {
		if raw, ok := changes[key]; ok {
			if json.Unmarshal(raw, ptr) != nil {
				return bad(400, "经纬度需填写数字")
			}
		}
	}
	if (c.Latitude == nil) != (c.Longitude == nil) {
		return bad(400, "请同时填写经度和纬度，或同时留空")
	}
	if c.Latitude != nil {
		lat, lng := *c.Latitude, *c.Longitude
		if math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lng) || math.IsInf(lng, 0) || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			return bad(400, "经纬度超出有效范围")
		}
		if c.Address == "" {
			return bad(400, "请填写公司地址")
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
			v, err := a.mediaValue(r.Context(), src, "image")
			if err != nil {
				return err
			}
			if v == "" {
				return bad(400, "请选择有效资质图片")
			}
			c.QualificationImages = append(c.QualificationImages, v)
		}
	}

	if raw, ok := changes["teamMembers"]; ok {
		var members []TeamMember
		if string(raw) == "null" || json.Unmarshal(raw, &members) != nil || len(members) > 50 {
			return bad(400, "团队成员需为列表，最多50人")
		}
		seen := map[string]bool{}
		for i := range members {
			m := &members[i]
			if m.ID == "" {
				m.ID, err = newID()
				if err != nil {
					return err
				}
			}
			if !idPattern.MatchString(m.ID) || seen[m.ID] {
				return bad(400, "团队成员标识无效或重复")
			}
			seen[m.ID] = true
			if m.Name, err = clean(m.Name, 80); err != nil {
				return err
			}
			if m.Role, err = clean(m.Role, 120); err != nil {
				return err
			}
			if m.Name == "" || m.Role == "" {
				return bad(400, "请填写成员姓名和职位")
			}
			if m.Group != "management" && m.Group != "execution" {
				return bad(400, "请选择管理团队或执行团队")
			}
			if m.Biography, err = clean(m.Biography, 5000); err != nil {
				return err
			}
			if m.Photo, err = a.mediaValue(r.Context(), m.Photo, "image"); err != nil {
				return err
			}

			if m.Phone, err = clean(m.Phone, 40); err != nil {
				return err
			}
			for _, ch := range m.Phone {
				if !strings.ContainsRune("0123456789+()- ", ch) {
					return bad(400, "成员电话格式不正确")
				}
			}
			if m.Wechat, err = clean(m.Wechat, 100); err != nil {
				return err
			}
			if strings.ContainsAny(m.Wechat, "\r\n") {
				return bad(400, "微信号不能换行")
			}
			if m.WechatQR, err = a.mediaValue(r.Context(), m.WechatQR, "image"); err != nil {
				return err
			}
			if err = validOrder(m.Order); err != nil {
				return err
			}
		}
		c.TeamMembers = members
	}
	if raw, ok := changes["homeFeaturedLimit"]; ok {
		if json.Unmarshal(raw, &c.HomeFeaturedLimit) != nil || c.HomeFeaturedLimit < 1 || c.HomeFeaturedLimit > 12 {
			return bad(400, "精选数量需为1至12")
		}
	}
	auditCompanyMembers(r, previousMembers, c.TeamMembers)
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
