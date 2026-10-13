package app

import (
	"gorm.io/gorm"
	"net/http"
	"time"
)

func (a *Server) normalizeCase(c Case, id string) (Case, error) {
	var err error
	c.ID = id
	c.Title, err = clean(c.Title, 120)
	if err != nil {
		return c, err
	}
	if c.Title == "" {
		return c, bad(400, "请填写案例名称")
	}
	if _, err = getRow[Category](a.Store.DB, "categories", c.CategoryID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return c, bad(400, "请选择有效业务分类")
		}
		return c, err
	}
	if c.ClientID != "" {
		if _, err = getRow[Client](a.Store.DB, "clients", c.ClientID); err != nil {
			if err == gorm.ErrRecordNotFound {
				return c, bad(400, "请选择有效客户")
			}
			return c, err
		}
	}
	switch c.Type {
	case "video", "graphic", "h5", "mixed":
	default:
		return c, bad(400, "请选择作品类型")
	}
	switch c.Status {
	case "draft", "published", "archived":
	default:
		return c, bad(400, "请选择发布状态")
	}
	if c.Cover, err = a.mediaValue(c.Cover, "image"); err != nil {
		return c, err
	}
	if c.Status == "published" && c.Cover == "" {
		return c, bad(400, "发布前请上传封面")
	}
	if len(c.Gallery) > 30 {
		return c, bad(400, "最多添加30张图片")
	}
	if c.Gallery == nil {
		c.Gallery = []string{}
	}
	for i, src := range c.Gallery {
		if c.Gallery[i], err = a.mediaValue(src, "image"); err != nil {
			return c, err
		}
	}
	if c.Video, err = a.mediaValue(c.Video, "video"); err != nil {
		return c, err
	}
	if c.ExperienceURL, err = safeLink(c.ExperienceURL); err != nil {
		return c, bad(400, err.Error())
	}
	if c.Summary, err = clean(c.Summary, 300); err != nil {
		return c, err
	}
	if c.Description, err = clean(c.Description, 10000); err != nil {
		return c, err
	}
	if c.Services, err = cleanList(c.Services, 30); err != nil {
		return c, err
	}
	if err = validOrder(c.Order); err != nil {
		return c, err
	}
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return c, nil
}
func (a *Server) saveCase(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	var err error
	if r.Method == "POST" {
		id, err = newID()
		if err != nil {
			return err
		}
	} else {
		if !idPattern.MatchString(id) {
			return bad(404, "案例不存在")
		}
		if _, err = getRow[Case](a.Store.DB, "cases", id); err != nil {
			return recordError(err, "案例不存在")
		}
	}
	var value Case
	if err = decode(w, r, &value); err != nil {
		return err
	}
	value, err = a.normalizeCase(value, id)
	if err != nil {
		return err
	}
	tx := a.Store.DB.Begin()
	if err = tx.Error; err != nil {
		return err
	}
	defer tx.Rollback()
	if value.Hero {
		items, err := listRows[Case](tx, "cases")
		if err != nil {
			return err
		}
		for _, c := range items {
			if c.ID != id && c.Hero {
				c.Hero = false
				if err = putRow(tx, "cases", c); err != nil {
					return err
				}
			}
		}
	}
	if err = putRow(tx, "cases", value); err != nil {
		return err
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	return respond(w, status, value)
}
func (a *Server) deleteCase(w http.ResponseWriter, r *http.Request) error {
	if err := deleteRow(a.Store.DB, "cases", r.PathValue("id")); err != nil {
		return recordError(err, "案例不存在")
	}
	return respond(w, 200, map[string]bool{"ok": true})
}
