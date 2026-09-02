// Package web wires the Gin router: middleware, the /api surface, and serving
// the embedded single-page frontend (with the secret admin path and the
// optional dedicated admin sub-domain).
package web

import (
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
}

type server struct {
	cfg *config.Config
	svc *service.Services
	log *logrus.Logger
}

// NewRouter builds the fully configured gin.Engine.
func NewRouter(opts Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	s := &server{cfg: opts.Config, svc: opts.Services, log: opts.Log}

	r := gin.New()
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	r.Use(s.recovery())
	r.Use(s.requestLogger())
	r.Use(s.securityHeaders())

	s.mountAPI(r)
	s.mountStatic(r) // must be last: it owns every non-/api path via NoRoute

	return r
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
	api.POST("/client-errors", s.handleReportClientError)

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
		admin.PUT("/config", s.handlePutConfig)

		admin.POST("/admin/logout", s.handleAdminLogout)
		admin.GET("/admin/backup", s.handleBackup)
		admin.POST("/admin/backup/restore", s.handleRestore)
		admin.POST("/admin/update", s.handleUpdate)

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
	}
}
