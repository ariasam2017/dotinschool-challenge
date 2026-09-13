package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GET /api/admin/system — the latest cached host CPU/RAM/Swap snapshot for the
// admin "منابع سرور" page. The sampling itself happens on a 1s background
// ticker (service.SysStatsService, started in cmd/server/main.go), so this
// handler just returns the cache — no per-request blocking — which is what
// lets the panel poll it every second or two for a near-real-time feel.
func (s *server) handleSystemStats(c *gin.Context) {
	c.JSON(http.StatusOK, s.svc.SysStats.Snapshot())
}
