package app

import (
	"encoding/json"
	"testing"
)

func TestCompanyLocationValidationAndClearing(t *testing.T) {
	h := newHarness(t)
	h.setup(t)
	status, config, _ := h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	c := config["company"].(map[string]any)
	if c["latitude"] != 39.993970 || c["longitude"] != 116.403885 {
		t.Fatal("location defaults missing", c)
	}
	for _, body := range []map[string]any{{"latitude": 91}, {"longitude": 181}, {"latitude": "39.9"}, {"latitude": nil}, {"longitude": nil}, {"address": ""}} {
		status, _, _ = h.call(t, "PUT", "/api/admin/company", body, true, nil)
		expect(t, status, 400)
	}
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]any{"latitude": 39.9, "longitude": 116.4}, true, nil)
	expect(t, status, 200)
	status, _, _ = h.call(t, "PUT", "/api/admin/company", map[string]any{"latitude": nil, "longitude": nil}, true, nil)
	expect(t, status, 200)
	if err := h.app.Store.seedCompanyLocation(); err != nil {
		t.Fatal(err)
	}
	status, config, _ = h.call(t, "GET", "/api/public/config", nil, false, nil)
	expect(t, status, 200)
	if config["company"].(map[string]any)["latitude"] != nil {
		t.Fatal("cleared location was repopulated")
	}
}

func TestLegacyCompanyLocationPreservesEdits(t *testing.T) {
	for _, scenario := range []string{"original", "address-edited", "coordinate-edited"} {
		t.Run(scenario, func(t *testing.T) {
			h := newHarness(t)
			record, err := getRow[map[string]json.RawMessage](h.app.Store.DB, "settings", "company")
			if err != nil {
				t.Fatal(err)
			}
			delete(record, "latitude")
			delete(record, "longitude")
			if scenario == "address-edited" {
				record["address"] = json.RawMessage(`"新地址"`)
			}
			if scenario == "coordinate-edited" {
				record["latitude"] = json.RawMessage(`39.8`)
				record["longitude"] = json.RawMessage(`116.5`)
			}
			if err = putRow(h.app.Store.DB, "settings", record); err != nil {
				t.Fatal(err)
			}
			if err = h.app.Store.seedCompanyLocation(); err != nil {
				t.Fatal(err)
			}
			c, err := getRow[Company](h.app.Store.DB, "settings", "company")
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "original" && (c.Latitude == nil || *c.Latitude != 39.993970) {
				t.Fatal("original address not initialized")
			}
			if scenario == "address-edited" && c.Latitude != nil {
				t.Fatal("changed address received wrong default coordinates")
			}
			if scenario == "coordinate-edited" && (c.Latitude == nil || *c.Latitude != 39.8) {
				t.Fatal("edited coordinates overwritten")
			}
		})
	}
}
