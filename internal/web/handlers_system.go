package web

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// meter is one resource gauge: bytes used / total and the derived percentage.
type meter struct {
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
}

// systemStatsResponse mirrors what the admin "منابع سرور" card renders — the
// same handful of numbers the 3x-ui / x-ui panel shows for the host machine.
type systemStatsResponse struct {
	CPU struct {
		Percent float64 `json:"percent"`
		Cores   int     `json:"cores"`
	} `json:"cpu"`
	Mem    meter      `json:"mem"`
	Swap   meter      `json:"swap"`
	Uptime uint64     `json:"uptime"` // host uptime, seconds
	Load   [3]float64 `json:"load"`   // 1/5/15-min load average (0 on Windows)
	Go     struct {
		Goroutines int    `json:"goroutines"`
		Alloc      uint64 `json:"alloc"` // this process's live heap, bytes
	} `json:"go"`
	At int64 `json:"at"` // epoch ms the sample was taken
}

// GET /api/admin/system — a live snapshot of host CPU / RAM / Swap for the
// overview panel. Every probe degrades gracefully: a failing collector just
// leaves its section zeroed rather than failing the whole request, so the card
// still renders on platforms where one metric is unavailable.
func (s *server) handleSystemStats(c *gin.Context) {
	var out systemStatsResponse
	out.At = nowMs()

	out.CPU.Cores = runtime.NumCPU()
	// A short blocking sample so the first call already returns a real number
	// (cpu.Percent(0, …) would report 0% until a second call).
	if pcts, err := cpu.Percent(150*time.Millisecond, false); err == nil && len(pcts) > 0 {
		out.CPU.Percent = round1(pcts[0])
	}

	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		out.Mem = meter{Used: vm.Used, Total: vm.Total, Percent: round1(vm.UsedPercent)}
	}

	if sw, err := mem.SwapMemory(); err == nil && sw != nil {
		out.Swap = meter{Used: sw.Used, Total: sw.Total, Percent: round1(sw.UsedPercent)}
	}

	if up, err := host.Uptime(); err == nil {
		out.Uptime = up
	}

	if avg, err := load.Avg(); err == nil && avg != nil {
		out.Load = [3]float64{round2(avg.Load1), round2(avg.Load5), round2(avg.Load15)}
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	out.Go.Goroutines = runtime.NumGoroutine()
	out.Go.Alloc = ms.Alloc

	c.JSON(http.StatusOK, out)
}

func round1(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
