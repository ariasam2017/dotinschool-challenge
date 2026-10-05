package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/apperr"
)

// securityHeaders sets the same hardening headers on every response — static
// assets included. connect-src/img-src/default-src pinned to 'self' keep an
// injected script from exfiltrating data to an outside origin even if one ever
// runs. HSTS is only sent when this process actually terminates TLS.
func (s *server) securityHeaders() gin.HandlerFunc {
	csp := "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"font-src 'self' data:; " +
		"connect-src 'self'; " +
		// 'self' (not 'none') so the admin Design Studio can embed the user
		// panel in a same-origin live-preview <iframe>. Cross-origin framing
		// stays blocked.
		"frame-ancestors 'self'; " +
		"base-uri 'self'; " +
		"form-action 'self'"

	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		// SAMEORIGIN (not DENY) for the Design Studio live-preview iframe.
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		h.Set("Content-Security-Policy", csp)
		if s.cfg.UseTLS {
			h.Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		}
		c.Next()
	}
}

// requestLogger is a compact logrus access log.
func (s *server) requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if (c.Request.URL.Path == "/api/config" || c.Request.URL.Path == "/api/admin/game/board") && c.Writer.Status() < 400 {
			return // the panels poll this every second — don't drown the log
		}
		s.log.WithFields(logrus.Fields{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"status": c.Writer.Status(),
			"ms":     time.Since(start).Milliseconds(),
			"ip":     c.ClientIP(),
		}).Debug("request")
	}
}

// recovery turns a panic into a generic 500 with no internal detail leaked.
func (s *server) recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				s.log.WithField("panic", r).WithField("path", c.Request.URL.Path).Error("recovered panic")
				if !c.Writer.Written() {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "خطای داخلی سرور رخ داد.",
					})
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}

// bodyLimit caps the number of bytes a request body may carry. The original
// server enforced a 1 MiB ceiling on *every* body; this rewrite had lost that,
// leaving an unauthenticated attacker able to exhaust memory by streaming a
// huge JSON payload to any POST/PATCH/PUT route. bindJSON turns the resulting
// error into a clean 413.
func bodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		}
		c.Next()
	}
}

// requireAdmin aborts unless a valid, unexpired admin bearer token is present.
// On expiry the token is dropped so a stale browser tab surfaces a clean 401.
func (s *server) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		username, ok := s.svc.Sessions.ResolveAdmin(bearerToken(c))
		if !ok {
			fail(c, apperr.Unauthorized())
			c.Abort()
			return
		}
		c.Set(ctxAdminUser, username)
		c.Next()
	}
}
