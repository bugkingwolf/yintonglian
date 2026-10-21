package app

import (
	"net/http"
	"time"
)

type diskUsage struct {
	TotalBytes     uint64  `json:"totalBytes"`
	UsedBytes      uint64  `json:"usedBytes"`
	AvailableBytes uint64  `json:"availableBytes"`
	UsedPercent    float64 `json:"usedPercent"`
	Level          string  `json:"level"`
	CheckedAt      string  `json:"checkedAt"`
}

// Count system-reserved space as occupied, matching the space available to uploads.
func summarizeDisk(total, available uint64) diskUsage {
	if available > total {
		available = total
	}
	d := diskUsage{TotalBytes: total, AvailableBytes: available, UsedBytes: total - available, Level: "normal", CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	if total > 0 {
		d.UsedPercent = float64(d.UsedBytes) / float64(total) * 100
	}
	if d.UsedPercent >= 90 {
		d.Level = "critical"
	} else if d.UsedPercent >= 80 {
		d.Level = "warning"
	}
	return d
}

func (a *Server) storageStatus(w http.ResponseWriter, r *http.Request) error {
	total, available, err := diskCapacity(a.Store.DataDir)
	if err != nil || total == 0 {
		return bad(503, "暂时无法读取磁盘容量，请稍后重试")
	}
	return respond(w, 200, summarizeDisk(total, available))
}
