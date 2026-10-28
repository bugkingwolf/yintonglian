package app

import (
	"fmt"
	"strings"
	"testing"
)

func TestCasePaginationSummariesFiltersAndLegacy(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	template := contentCase(t, h, "life-forward")
	for i := 0; i < 37; i++ {
		c := template
		c.ID = fmt.Sprintf("paging-%03d", i)
		c.Title = fmt.Sprintf("分页案例 %03d", i)
		c.Order = 500
		c.Description = "only-in-detail"
		c.Gallery = []string{"/assets/life-forward.jpg"}
		c.Status = "published"
		c.Hero = false
		c.Featured = false
		if i == 36 {
			c.Status = "draft"
		}
		if err := putRow(h.app.Store.DB, "cases", c); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for page := 1; page <= 4; page++ {
		status, v, _ := h.call(t, "GET", fmt.Sprintf("/api/public/case-list?page=%d&pageSize=12", page), nil, false, nil)
		expect(t, status, 200)
		if v["total"] != float64(42) {
			t.Fatal(v["total"])
		}
		for _, raw := range v["items"].([]any) {
			x := raw.(map[string]any)
			id := x["id"].(string)
			if seen[id] {
				t.Fatal("duplicate", id)
			}
			seen[id] = true
			for _, key := range []string{"description", "gallery", "video", "experienceUrl", "services"} {
				if _, ok := x[key]; ok {
					t.Fatal("detail leaked", key)
				}
			}
		}
	}
	if len(seen) != 42 || seen["paging-036"] {
		t.Fatal(len(seen))
	}
	status, v, _ := h.call(t, "GET", "/api/public/case-list?q=only-in-detail&pageSize=100", nil, false, nil)
	expect(t, status, 200)
	if v["total"] != float64(36) {
		t.Fatal(v)
	}
	status, v, _ = h.call(t, "GET", "/api/admin/cases?q=only-in-detail&status=draft", nil, true, nil)
	expect(t, status, 200)
	if v["total"] != float64(1) {
		t.Fatal(v)
	}
	status, v, _ = h.call(t, "GET", "/api/admin/cases/paging-036", nil, true, nil)
	expect(t, status, 200)
	if v["description"] != "only-in-detail" {
		t.Fatal(v)
	}
	status, _, _ = h.call(t, "GET", "/api/public/cases/paging-036", nil, false, nil)
	expect(t, status, 404)
	for _, path := range []string{"/api/public/cases", "/api/admin/content"} {
		for _, auth := range []bool{false, true} {
			status, _, _ = h.call(t, "GET", path, nil, auth, nil)
			expect(t, status, 404)
		}
	}
	for _, q := range []string{"page=0", "pageSize=101", "type=sql", "status=bad"} {
		path := "/api/public/case-list?" + q
		auth := false
		if strings.HasPrefix(q, "status") {
			path = "/api/admin/cases?" + q
			auth = true
		}
		status, _, _ = h.call(t, "GET", path, nil, auth, nil)
		expect(t, status, 400)
	}
	status, v, _ = h.call(t, "GET", "/api/public/case-list?page=999&pageSize=12", nil, false, nil)
	expect(t, status, 200)
	if v["page"] != float64(4) {
		t.Fatal(v)
	}
}
func TestPagedMetadataHomeAndUploads(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	status, v, _ := h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	if _, ok := v["cases"]; ok {
		t.Fatal("cases in config")
	}
	status, v, _ = h.call(t, "GET", "/api/admin/config", nil, true, nil)
	expect(t, status, 200)
	if _, ok := v["cases"]; ok {
		t.Fatal("cases in admin config")
	}
	if _, ok := v["uploads"]; ok {
		t.Fatal("uploads in config")
	}
	if v["stats"].(map[string]any)["total"] != float64(6) {
		t.Fatal(v)
	}
	status, v, _ = h.call(t, "GET", "/api/public/home", nil, false, nil)
	expect(t, status, 200)
	if len(v["hero"].([]any)) != 1 || len(v["featured"].([]any)) > 12 || len(v["categoryCases"].([]any)) > 3 {
		t.Fatal(v)
	}
	for i := 0; i < 29; i++ {
		x := Upload{ID: fmt.Sprintf("upload-%03d", i), Filename: fmt.Sprintf("upload-%03d.png", i), MIME: "image/png", OriginalName: fmt.Sprintf("图片%03d", i), CreatedAt: 100}
		if err := h.app.Store.DB.Create(&x).Error; err != nil {
			t.Fatal(err)
		}
	}
	for page, n := range map[int]int{1: 24, 2: 5} {
		status, v, _ = h.call(t, "GET", fmt.Sprintf("/api/admin/uploads?page=%d", page), nil, true, nil)
		expect(t, status, 200)
		if len(v["items"].([]any)) != n || v["total"] != float64(29) {
			t.Fatal(v)
		}
	}
	status, v, _ = h.call(t, "GET", "/api/admin/uploads?kind=video", nil, true, nil)
	expect(t, status, 200)
	if v["total"] != float64(0) {
		t.Fatal(v)
	}
	status, v, _ = h.call(t, "GET", "/api/admin/uploads?q=028", nil, true, nil)
	expect(t, status, 200)
	if v["total"] != float64(1) {
		t.Fatal(v)
	}
	status, _, _ = h.call(t, "GET", "/api/admin/uploads", nil, false, nil)
	expect(t, status, 401)
}
