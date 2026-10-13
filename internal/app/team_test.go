package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTeamVisibilityValidationAndMedia(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	status, public, _ := h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	if len(public["company"].(map[string]any)["teamMembers"].([]any)) != 12 {
		t.Fatal("PPT team should contain 12 members")
	}
	upload := uploadImage(t, h)
	photo := upload["url"].(string)
	members := []TeamMember{{ID: "visible-member", Name: "公开成员", Role: "导演", Group: "management", Photo: photo, Order: 20, Enabled: true}, {ID: "hidden-member", Name: "隐藏成员", Role: "策划", Group: "execution", Order: 10, Enabled: false}}
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": members}, true, nil)
	expect(t, status, 200)
	status, public, _ = h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	exposed := public["company"].(map[string]any)["teamMembers"].([]any)
	if len(exposed) != 1 || exposed[0].(map[string]any)["name"] != "公开成员" {
		t.Fatal("hidden member leaked")
	}
	status, _, _ = h.call(t, "GET", photo, nil, false, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+upload["id"].(string), nil, true, nil)
	expect(t, status, 409)
	members[0].Enabled = false
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": members}, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "GET", photo, nil, false, nil)
	expect(t, status, 404)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+upload["id"].(string), nil, true, nil)
	expect(t, status, 409)
	members[0].Photo = "https://example.com/unsafe.jpg"
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": members}, true, nil)
	expect(t, status, 400)
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": nil}, true, nil)
	expect(t, status, 400)
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]any{"teamMembers": []TeamMember{}}, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "DELETE", "/api/admin/uploads/"+upload["id"].(string), nil, true, nil)
	expect(t, status, 200)
}

func TestLegacyTeamInitializationPreservesEdits(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	record, err := getRow[map[string]json.RawMessage](s.DB, "settings", "company")
	if err != nil {
		t.Fatal(err)
	}
	delete(record, "teamMembers")
	record["name"] = json.RawMessage(`"保留的公司名称"`)
	if err = putRow(s.DB, "settings", record); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := getRow[Company](s.DB, "settings", "company")
	if err != nil || c.Name != "保留的公司名称" || len(c.TeamMembers) != 12 {
		t.Fatalf("migration %+v %v", c, err)
	}
	for _, m := range c.TeamMembers {
		if _, err = os.Stat(filepath.Join(projectRoot(), "public", m.Photo)); err != nil {
			t.Fatal(err)
		}
	}
	c.TeamMembers = []TeamMember{}
	if err = putRow(s.DB, "settings", c); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err = getRow[Company](s.DB, "settings", "company")
	if err != nil || len(c.TeamMembers) != 0 {
		t.Fatal("explicit empty team refilled")
	}
}
