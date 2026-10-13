// Package web wires the Gin router: middleware, the /api surface, and serving
// the embedded single-page frontend (with the secret admin path and the
// optional dedicated admin sub-domain).
package web

import (
	"net"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/config"
	"github.com/dotinschool/dotinschool/internal/service"
)

// Options are the dependencies NewRouter needs.
type Options struct {
	Config   *config.Config
	Services *service.Services
	Log      *logrus.Logger
	Version  string // build version stamped via -ldflags "-X main.version=..."
}

// Request body ceilings. The original Node server capped every body at 1 MiB;
// this is a little more generous but still bounded. Restore is the one route
// that legitimately needs a large body (a base64 SQLite file).
const (
	defaultBodyLimit = 2 << 20   // 2 MiB
	restoreBodyLimit = 256 << 20 // 256 MiB
	// a bulk question-bank import (up to 5000 questions)
	questionImportLimit = 16 << 20 // 16 MiB
)

type server struct {
	cfg     *config.Config
	svc     *service.Services
	log     *logrus.Logger
	version string
}

// NewRouter builds the fully configured gin.Engine.
func NewRouter(opts Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	s := &server{cfg: opts.Config, svc: opts.Services, log: opts.Log, version: firstNonEmpty(opts.Version, "dev")}

	r := gin.New()
	configureProxyTrust(r, opts.Config.TrustedProxies)

	r.Use(s.recovery())
	r.Use(s.requestLogger())
	r.Use(s.securityHeaders())
	r.Use(bodyLimit(defaultBodyLimit)) // 2 MiB ceiling on every request body

	r.MaxMultipartMemory = 4 << 20

	s.mountAPI(r)
	s.mountStatic(r) // must be last: it owns every non-/api path via NoRoute

	return r
}

// configureProxyTrust decides where c.ClientIP() comes from. Loopback is always
// a trusted proxy (a local Caddy, or cloudflared for "dotinschool tunnel").
// Behind the Cloudflare Tunnel every request arrives from cloudflared on
// loopback and CF-Connecting-IP carries the real visitor — without it, all VPN
// players would share one IP for rate limiting and fail2ban. extra (the
// TRUSTED_PROXIES env) adds the Docker network a Caddy container reaches the
// app from; without it every visitor behind that Caddy shares Caddy's address,
// so one attacker could exhaust the admin-login / OTP limits for everyone.
// Forwarding headers are ignored from anyone else, so a direct visitor cannot
// spoof their IP with them.
func configureProxyTrust(r *gin.Engine, extra []string) {
	trusted := []string{"127.0.0.1", "::1"}
	for _, p := range extra {
		if _, _, err := net.ParseCIDR(p); err == nil || net.ParseIP(p) != nil {
			trusted = append(trusted, p)
		}
	}
	_ = r.SetTrustedProxies(trusted)
	r.RemoteIPHeaders = []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"}
}

// mountAPI registers every /api/* route with the exact method, path, auth, and
// rate-limit rules of the original server.
func (s *server) mountAPI(r *gin.Engine) {
	api := r.Group("/api")

	// ---- public (rate-limited) ----
	api.POST("/otp/request", s.handleOTPRequest)
	api.POST("/register", s.handleRegister)
	api.POST("/lookup", s.handleLookup)
	api.POST("/session", s.handleSession)
	api.GET("/config", s.handleGetConfig)
	api.GET("/version", s.handleVersion)
	api.POST("/client-errors", s.handleReportClientError)

	// ---- quiz game player (token in body) ----
	api.GET("/game/info", s.handleGameInfo)
	api.POST("/game/register", s.handleGameRegister)
	api.POST("/game/state", s.handleGameState)
	api.POST("/game/start", s.handleGameStart)
	api.POST("/game/answer", s.handleGameAnswer)
	api.POST("/game/next", s.handleGameNext)
	api.POST("/game/replay", s.handleGameReplay)

	// ---- participant OR admin ----
	api.PATCH("/participants/:id", s.handlePatchParticipant)

	// ---- admin only ----
	admin := api.Group("")
	{
		api.POST("/admin/login", s.handleAdminLogin) // login itself is unauthenticated
	}
	admin.Use(s.requireAdmin())
	{
		admin.GET("/participants", s.handleListParticipants)
		admin.DELETE("/participants/:id", s.handleDeleteParticipant)
		admin.GET("/admin/participants/export", s.handleExportParticipants)
		admin.POST("/admin/participants/import", bodyLimit(restoreBodyLimit), s.handleImportParticipants)
		admin.PUT("/config", s.handlePutConfig)

		admin.POST("/admin/logout", s.handleAdminLogout)
		admin.GET("/admin/backup", s.handleBackup)
		// restore uploads a whole base64-encoded SQLite file — needs a much
		// higher ceiling than the 2 MiB default (still admin-only + magic-checked).
		admin.POST("/admin/backup/restore", bodyLimit(restoreBodyLimit), s.handleRestore)
		admin.POST("/admin/update", s.handleUpdate)

		admin.GET("/admin/appearance", s.handleGetAppearance)
		admin.PUT("/admin/appearance", s.handlePutAppearanceDraft)
		admin.POST("/admin/appearance/publish", s.handlePublishAppearance)
		admin.POST("/admin/appearance/revert", s.handleRevertAppearance)
		admin.POST("/admin/appearance/reset", s.handleResetAppearance)

		admin.GET("/admin/ai-status", s.handleAIStatus)
		admin.GET("/admin/ai-settings", s.handleGetAISettings)
		admin.PUT("/admin/ai-settings", s.handlePutAISettings)
		admin.POST("/admin/ai-settings/test", s.handleTestAIProvider)
		admin.POST("/admin/ai-detect", s.handleDetectAIProvider)
		admin.POST("/admin/review-code", s.handleReviewCode)

		admin.GET("/admin/client-errors", s.handleListClientErrors)
		admin.POST("/admin/client-errors/seen", s.handleSeenClientErrors)
		admin.DELETE("/admin/client-errors/:id", s.handleDeleteClientError)
		admin.DELETE("/admin/client-errors", s.handleClearClientErrors)

		admin.GET("/admin/system", s.handleSystemStats)

		admin.GET("/admin/sms-status", s.handleSMSStatus)
		admin.POST("/admin/sms-test", s.handleSMSTest)
		admin.GET("/admin/game/board", s.handleGameBoard)
		admin.GET("/admin/game/report", s.handleGameReport)
		admin.PUT("/admin/game/settings", s.handlePutGameSettings)
		admin.GET("/admin/game/players/:id", s.handleGamePlayerReview)
		admin.POST("/admin/game/players/:id/reset", s.handleGameResetPlayer)
		admin.DELETE("/admin/game/players/:id", s.handleGameDeletePlayer)
		admin.DELETE("/admin/game/players", s.handleGameDeleteAllPlayers)
		admin.GET("/admin/game/questions", s.handleListGameQuestions)
		admin.POST("/admin/game/questions", s.handleCreateGameQuestion)
		admin.PUT("/admin/game/questions/:id", s.handleUpdateGameQuestion)
		admin.DELETE("/admin/game/questions/:id", s.handleDeleteGameQuestion)
		admin.POST("/admin/game/questions/delete", s.handleBulkDeleteGameQuestions)
		admin.POST("/admin/game/questions/import", bodyLimit(questionImportLimit), s.handleImportGameQuestions)
		admin.POST("/admin/game/questions/defaults", s.handleAddDefaultGameQuestions)

		admin.GET("/admin/sms-settings", s.handleGetSMSSettings)
		admin.PUT("/admin/sms-settings", s.handlePutSMSSettings)
	}
}
