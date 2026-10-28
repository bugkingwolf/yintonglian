package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type CaseSummary struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	ClientID   string `json:"clientId"`
	CategoryID string `json:"categoryId"`
	Type       string `json:"type"`
	Summary    string `json:"summary"`
	Cover      string `json:"cover"`
	HasVideo   bool   `json:"hasVideo"`
	Featured   bool   `json:"featured"`
	Hero       bool   `json:"hero"`
	Demo       bool   `json:"demo"`
	Status     string `json:"status"`
	Order      int    `json:"order"`
	UpdatedAt  string `json:"updatedAt"`
}
type casePage struct {
	Items    []CaseSummary `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

const caseOrder = "CAST(json_extract(cases.payload, '$.order') AS INTEGER) ASC, cases.id ASC"
const summaryProjection = `json_object('id',cases.id,'title',json_extract(cases.payload,'$.title'),'clientId',coalesce(cases.client_id,''),'categoryId',cases.category_id,'type',json_extract(cases.payload,'$.type'),'summary',json_extract(cases.payload,'$.summary'),'cover',json_extract(cases.payload,'$.cover'),'hasVideo',json(CASE WHEN coalesce(json_extract(cases.payload,'$.video'),'') != '' THEN 'true' ELSE 'false' END),'featured',json(CASE WHEN json_extract(cases.payload,'$.featured')=1 THEN 'true' ELSE 'false' END),'hero',json(CASE WHEN json_extract(cases.payload,'$.hero')=1 THEN 'true' ELSE 'false' END),'demo',json(CASE WHEN json_extract(cases.payload,'$.demo')=1 THEN 'true' ELSE 'false' END),'status',cases.status,'order',json_extract(cases.payload,'$.order'),'updatedAt',json_extract(cases.payload,'$.updatedAt')) AS payload`

func pageParams(q url.Values, defaultSize int) (int, int, error) {
	page, size := 1, defaultSize
	for _, v := range []struct {
		key  string
		dest *int
		max  int
	}{{"page", &page, 1000000}, {"pageSize", &size, 100}} {
		if text := q.Get(v.key); text != "" {
			n, e := strconv.Atoi(text)
			if e != nil || n < 1 || n > v.max {
				return 0, 0, bad(400, "分页参数不正确")
			}
			*v.dest = n
		}
	}
	return page, size, nil
}
func (a *Server) caseQuery(q url.Values, admin bool) (*gorm.DB, error) {
	db := a.Store.DB.Table("cases")
	if !admin {
		db = db.Where("cases.status = ?", "published")
	} else if status := q.Get("status"); status != "" {
		if status != "published" && status != "draft" && status != "archived" {
			return nil, bad(400, "发布状态不正确")
		}
		db = db.Where("cases.status = ?", status)
	}
	for _, filter := range []struct{ key, column string }{{"category", "category_id"}, {"client", "client_id"}} {
		if v := q.Get(filter.key); v != "" {
			if !idPattern.MatchString(v) {
				return nil, bad(400, "筛选标识不正确")
			}
			db = db.Where("cases."+filter.column+" = ?", v)
		}
	}
	if v := q.Get("type"); v != "" {
		if v != "video" && v != "graphic" && v != "h5" && v != "mixed" {
			return nil, bad(400, "作品类型不正确")
		}
		db = db.Where("json_extract(cases.payload,'$.type') = ?", v)
	}
	for _, field := range []string{"featured", "hero"} {
		if v := q.Get(field); v != "" {
			if v != "1" {
				return nil, bad(400, "推荐筛选不正确")
			}
			db = db.Where("json_extract(cases.payload,'$." + field + "') = 1")
		}
	}
	if search := strings.ToLower(strings.TrimSpace(q.Get("q"))); search != "" {
		if len([]rune(search)) > 120 {
			return nil, bad(400, "搜索内容过长")
		}
		db = db.Where(`instr(lower(coalesce(json_extract(cases.payload,'$.title'),'') || ' ' || coalesce(json_extract(cases.payload,'$.summary'),'') || ' ' || coalesce(json_extract(cases.payload,'$.description'),'') || ' ' || coalesce((SELECT json_extract(clients.payload,'$.name') FROM clients WHERE clients.id=cases.client_id),'')), ?) > 0`, search)
	}
	if id := q.Get("relatedTo"); id != "" {
		c, err := getRow[Case](a.Store.DB, "cases", id)
		if err != nil || c.Status != "published" {
			return nil, bad(404, "案例尚未发布或已下架")
		}
		db = db.Where("cases.id <> ?", id)
		if c.ClientID != "" {
			db = db.Where("(cases.category_id = ? OR cases.client_id = ?)", c.CategoryID, c.ClientID)
		} else {
			db = db.Where("cases.category_id = ?", c.CategoryID)
		}
	}
	return db, nil
}
func (a *Server) queryCasePage(q url.Values, admin bool) (casePage, error) {
	p, size, err := pageParams(q, 12)
	out := casePage{Items: []CaseSummary{}, Page: p, PageSize: size}
	if err != nil {
		return out, err
	}
	db, err := a.caseQuery(q, admin)
	if err != nil {
		return out, err
	}
	if err = db.Count(&out.Total).Error; err != nil {
		return out, err
	}
	last := max(1, int((out.Total+int64(size)-1)/int64(size)))
	if out.Page > last {
		out.Page = last
	}
	var records []contentRecord
	if err = db.Select(summaryProjection).Order(caseOrder).Limit(size).Offset((out.Page - 1) * size).Find(&records).Error; err != nil {
		return out, err
	}
	for _, r := range records {
		var c CaseSummary
		if err = json.Unmarshal([]byte(r.Payload), &c); err != nil {
			return out, err
		}
		out.Items = append(out.Items, c)
	}
	return out, nil
}
func (a *Server) caseList(admin bool) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		out, err := a.queryCasePage(r.URL.Query(), admin)
		if err != nil {
			return err
		}
		return respond(w, 200, out)
	}
}
func (a *Server) adminCase(w http.ResponseWriter, r *http.Request) error {
	c, err := getRow[Case](a.Store.DB, "cases", r.PathValue("id"))
	if err != nil {
		return recordError(err, "案例不存在")
	}
	return respond(w, 200, c)
}

// Metadata intentionally excludes all case payloads and uploaded-file records.
func (s *Store) Metadata() (Snapshot, error) {
	var d Snapshot
	var err error
	if d.Categories, err = listRows[Category](s.DB, "categories"); err != nil {
		return d, err
	}
	if d.Clients, err = listRows[Client](s.DB, "clients"); err != nil {
		return d, err
	}
	if d.Company, err = getRow[Company](s.DB, "settings", "company"); err != nil {
		return d, err
	}
	p, err := getRow[platformSetting](s.DB, "settings", "platforms")
	if err != nil {
		return d, err
	}
	d.Platforms = p.Items
	sort.SliceStable(d.Categories, func(i, j int) bool { return d.Categories[i].Order < d.Categories[j].Order })
	sort.SliceStable(d.Clients, func(i, j int) bool { return d.Clients[i].Order < d.Clients[j].Order })
	sort.SliceStable(d.Platforms, func(i, j int) bool { return d.Platforms[i].Order < d.Platforms[j].Order })
	return d, nil
}
func (a *Server) publicHome(w http.ResponseWriter, r *http.Request) error {
	company, err := getRow[Company](a.Store.DB, "settings", "company")
	if err != nil {
		return err
	}
	hero, err := a.queryCasePage(url.Values{"hero": {"1"}, "pageSize": {"1"}}, false)
	if err != nil {
		return err
	}
	if len(hero.Items) == 0 {
		hero, err = a.queryCasePage(url.Values{"pageSize": {"1"}}, false)
		if err != nil {
			return err
		}
	}
	featured, err := a.queryCasePage(url.Values{"featured": {"1"}, "pageSize": {strconv.Itoa(min(12, max(1, company.HomeFeaturedLimit)))}}, false)
	if err != nil {
		return err
	}
	categories, err := listRows[Category](a.Store.DB, "categories")
	if err != nil {
		return err
	}
	previews := []CaseSummary{}
	for _, c := range categories {
		p, e := a.queryCasePage(url.Values{"category": {c.ID}, "pageSize": {"1"}}, false)
		if e != nil {
			return e
		}
		previews = append(previews, p.Items...)
	}
	var clientIDs []string
	if err = a.Store.DB.Table("cases").Where("status = ? AND client_id IS NOT NULL", "published").Distinct("client_id").Pluck("client_id", &clientIDs).Error; err != nil {
		return err
	}
	return respond(w, 200, map[string]any{"hero": hero.Items, "featured": featured.Items, "categoryCases": previews, "clientIds": clientIDs})
}
func (a *Server) adminConfig(w http.ResponseWriter, r *http.Request) error {
	d, err := a.Store.Metadata()
	if err != nil {
		return err
	}
	var stats struct {
		Total       int64 `json:"total"`
		Published   int64 `json:"published"`
		Unpublished int64 `json:"unpublished"`
		Featured    int64 `json:"featured"`
	}
	if err = a.Store.DB.Table("cases").Select("count(*) AS total, coalesce(sum(status='published'),0) AS published, coalesce(sum(status<>'published'),0) AS unpublished, coalesce(sum(status='published' AND json_extract(payload,'$.featured')=1),0) AS featured").Scan(&stats).Error; err != nil {
		return err
	}
	counts := map[string]map[string]int64{"categories": {}, "clients": {}}
	for _, group := range []struct{ name, column string }{{"categories", "category_id"}, {"clients", "client_id"}} {
		var rows []struct {
			ID    string
			Count int64
		}
		if err = a.Store.DB.Table("cases").Select(group.column + " AS id,count(*) AS count").Group(group.column).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			counts[group.name][row.ID] = row.Count
		}
	}
	return respond(w, 200, map[string]any{"company": d.Company, "categories": d.Categories, "clients": d.Clients, "platforms": d.Platforms, "stats": stats, "counts": counts})
}
func (a *Server) uploadList(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	page, size, err := pageParams(q, 24)
	if err != nil {
		return err
	}
	db := a.Store.DB.Model(&Upload{})
	if kind := q.Get("kind"); kind != "" {
		if kind != "image" && kind != "video" {
			return bad(400, "素材类型不正确")
		}
		db = db.Where("mime LIKE ?", kind+"/%")
	}
	if search := q.Get("q"); search != "" {
		if len([]rune(search)) > 120 {
			return bad(400, "搜索内容过长")
		}
		db = db.Where("instr(lower(original_name),?) > 0", strings.ToLower(search))
	}
	var total int64
	if err = db.Count(&total).Error; err != nil {
		return err
	}
	last := max(1, int((total+int64(size)-1)/int64(size)))
	page = min(page, last)
	items := []Upload{}
	if err = db.Order("created_at DESC, id DESC").Limit(size).Offset((page - 1) * size).Find(&items).Error; err != nil {
		return err
	}
	return respond(w, 200, map[string]any{"items": items, "total": total, "page": page, "pageSize": size})
}
