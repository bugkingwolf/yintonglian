package app

import (
	"net/http"
	"sort"
)

func (a *Server) publicConfig(w http.ResponseWriter, r *http.Request) error {
	d, err := a.Store.Metadata()
	if err != nil {
		return err
	}
	enabled := make([]Platform, 0)
	for _, p := range d.Platforms {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	members := make([]TeamMember, 0)
	for _, m := range d.Company.TeamMembers {
		if m.Enabled {
			members = append(members, m)
		}
	}
	sort.SliceStable(members, func(i, j int) bool { return members[i].Order < members[j].Order })
	d.Company.TeamMembers = members
	return respond(w, 200, map[string]any{"categories": d.Categories, "clients": d.Clients, "company": d.Company, "platforms": enabled})
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
