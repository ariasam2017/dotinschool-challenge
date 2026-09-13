package service

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// SysMeter is one resource gauge: bytes used / total and the derived percentage.
type SysMeter struct {
	Used    uint64  `json:"used"`
	Total   uint64  `json:"total"`
	Percent float64 `json:"percent"`
}

// SysSnapshot is a point-in-time reading of host resources — the exact JSON the
// admin "منابع سرور" page renders.
type SysSnapshot struct {
	CPU struct {
		Percent float64 `json:"percent"`
		Cores   int     `json:"cores"`
	} `json:"cpu"`
	Mem    SysMeter   `json:"mem"`
	Swap   SysMeter   `json:"swap"`
	Uptime uint64     `json:"uptime"` // host uptime, seconds
	Load   [3]float64 `json:"load"`   // 1/5/15-min load average (0 on Windows)
	Go     struct {
		Goroutines int    `json:"goroutines"`
		Alloc      uint64 `json:"alloc"` // this process's live heap, bytes
	} `json:"go"`
	At int64 `json:"at"` // epoch ms the sample was taken
}

// SysStatsService keeps a fresh host-resource snapshot sampled on a background
// ticker, so GET /api/admin/system answers instantly from cache instead of
// blocking ~150ms to sample the CPU on every request. That is what lets the
// panel poll it every second or two for a genuinely near-real-time feel
// instead of a request-latency-bound one.
type SysStatsService struct {
	mu   sync.RWMutex
	snap SysSnapshot
	stop chan struct{}
	once sync.Once
}

func newSysStatsService() *SysStatsService {
	s := &SysStatsService{stop: make(chan struct{})}
	s.sample() // prime cpu.Percent's baseline + fill mem/swap right away
	return s
}

// Start begins sampling every interval (idempotent — safe to call more than
// once). Call it once from the server process; cmd/seed and tests can leave
// the service unstarted and just read the one primed sample.
func (s *SysStatsService) Start(interval time.Duration) {
	s.once.Do(func() {
		go func() {
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				select {
				case <-s.stop:
					return
				case <-t.C:
					s.sample()
				}
			}
		}()
	})
}

// Stop ends the sampling goroutine.
func (s *SysStatsService) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

// Snapshot returns the most recent reading (a value copy — safe to hand out).
func (s *SysStatsService) Snapshot() SysSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

// sample reads every collector once and stores the result. A failing collector
// just leaves its section zeroed (e.g. load average on Windows) instead of
// failing the whole snapshot.
func (s *SysStatsService) sample() {
	var out SysSnapshot
	out.At = time.Now().UnixMilli()
	out.CPU.Cores = runtime.NumCPU()

	// interval=0 -> percent since the previous cpu.Percent call, i.e. over one
	// ticker period. Non-blocking, unlike sampling over a fixed window.
	if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
		out.CPU.Percent = round1s(p[0])
	}
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		out.Mem = SysMeter{Used: vm.Used, Total: vm.Total, Percent: round1s(vm.UsedPercent)}
	}
	if sw, err := mem.SwapMemory(); err == nil && sw != nil {
		out.Swap = SysMeter{Used: sw.Used, Total: sw.Total, Percent: round1s(sw.UsedPercent)}
	}
	if up, err := host.Uptime(); err == nil {
		out.Uptime = up
	}
	if avg, err := load.Avg(); err == nil && avg != nil {
		out.Load = [3]float64{round2s(avg.Load1), round2s(avg.Load5), round2s(avg.Load15)}
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	out.Go.Goroutines = runtime.NumGoroutine()
	out.Go.Alloc = ms.Alloc

	s.mu.Lock()
	s.snap = out
	s.mu.Unlock()
}

func round1s(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }
func round2s(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }
