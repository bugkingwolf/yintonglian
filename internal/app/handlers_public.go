package app

import (
	"net/http"
	"sort"
	"strings"
)

func (a *Server) publicConfig(w http.ResponseWriter, r *http.Request) error {
	d, err := a.Store.Snapshot()
	if err != nil {
		return err
	}
	enabled := make([]Platform, 0)
	for _, p := range d.Platforms {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	return respond(w, 200, map[string]any{"categories": d.Categories, "clients": d.Clients, "company": d.Company, "platforms": enabled})
}
func (a *Server) published() ([]Case, error) {
	items, err := listRows[Case](a.Store.DB, "cases")
	if err != nil {
		return nil, err
	}
	out := make([]Case, 0)
	for _, c := range items {
		if c.Status == "published" {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out, nil
}
func (a *Server) publicCases(w http.ResponseWriter, r *http.Request) error {
	items, err := a.published()
	if err != nil {
		return err
	}
	out := make([]Case, 0)
	q := r.URL.Query()
	search := strings.ToLower(q.Get("q"))
	if len(search) > 480 {
		search = search[:480]
	}
	for _, c := range items {
		if q.Get("category") != "" && c.CategoryID != q.Get("category") {
			continue
		}
		if q.Get("client") != "" && c.ClientID != q.Get("client") {
			continue
		}
		if q.Get("type") != "" && c.Type != q.Get("type") {
			continue
		}
		if q.Get("featured") == "1" && !c.Featured {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(c.Title+" "+c.Summary+" "+c.Description), search) {
			continue
		}
		out = append(out, c)
	}
	return respond(w, 200, map[string]any{"items": out})
}
func (a *Server) publicCase(w http.ResponseWriter, r *http.Request) error {
	c, err := getRow[Case](a.Store.DB, "cases", r.PathValue("id"))
	if err != nil {
		return recordError(err, "案例尚未发布或已下架")
	}
	if c.Status != "published" {
		return bad(404, "案例尚未发布或已下架")
	}
	return respond(w, 200, c)
}
func (a *Server) adminContent(w http.ResponseWriter, r *http.Request) error {
	d, err := a.Store.Snapshot()
	if err != nil {
		return err
	}
	return respond(w, 200, d)
}
