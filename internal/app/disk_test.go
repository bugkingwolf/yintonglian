package app

import "testing"

func TestDiskThresholds(t *testing.T) {
	for _, tc := range []struct {
		available uint64
		level     string
	}{{21, "normal"}, {20, "warning"}, {11, "warning"}, {10, "critical"}, {0, "critical"}, {101, "normal"}} {
		d := summarizeDisk(100, tc.available)
		if d.Level != tc.level || d.UsedBytes+d.AvailableBytes != d.TotalBytes || d.UsedPercent < 0 || d.UsedPercent > 100 {
			t.Fatalf("bad summary: %+v", d)
		}
	}
}

func TestStorageStatusRequiresLogin(t *testing.T) {
	h := newHarness(t)
	status, _, _ := h.call(t, "GET", "/api/admin/storage", nil, false, nil)
	expect(t, status, 401)
	h.setup(t)
	status, v, res := h.call(t, "GET", "/api/admin/storage", nil, true, nil)
	expect(t, status, 200)
	total, available, used := v["totalBytes"].(float64), v["availableBytes"].(float64), v["usedBytes"].(float64)
	if total <= 0 || available < 0 || total != available+used {
		t.Fatalf("bad capacity: %v", v)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("capacity must not be cached")
	}
	if _, ok := v["path"]; ok {
		t.Fatal("data path leaked")
	}
}
