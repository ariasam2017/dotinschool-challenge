package web

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
	"github.com/dotinschool/dotinschool/internal/service"
)

// POST /api/admin/login — public, rate-limited. Timing-safe credential check;
// issues a 12h bearer token.
func (s *server) handleAdminLogin(c *gin.Context) {
	ip := c.ClientIP()
	if !s.svc.RateLimiter.Allow("admin-login", ip, 10, 15*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}

	ok := service.TimingSafeEqual(body.Username, s.cfg.AdminUsername) &&
		service.TimingSafeEqual(body.Password, s.cfg.AdminPassword)
	if !ok {
		// Always-on (not gated by LOG_LEVEL), stable format so fail2ban can
		// match it: `msg="admin login failed" ip=<IP>` — see scripts/install.sh.
		s.log.WithField("ip", ip).Warn("admin login failed")
		fail(c, apperr.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "نام کاربری یا رمز عبور اشتباه است."))
		return
	}

	s.svc.RateLimiter.Clear("admin-login", ip)
	token := s.svc.Sessions.IssueAdmin(body.Username)
	c.JSON(http.StatusOK, gin.H{"token": token, "username": body.Username})
}

// POST /api/admin/logout — invalidates the current token.
func (s *server) handleAdminLogout(c *gin.Context) {
	s.svc.Sessions.RevokeAdmin(bearerToken(c))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

var challengeImageRe = regexp.MustCompile(`^data:image/(png|jpe?g|webp|gif);base64,`)

// PUT /api/config — admin. Persists any subset of the event/challenge fields.
func (s *server) handlePutConfig(c *gin.Context) {
	var body struct {
		EventEndsAt    json.RawMessage `json:"eventEndsAt"`
		EventStartsAt  json.RawMessage `json:"eventStartsAt"`
		EventPaused    *bool           `json:"eventPaused"`
		PausedRemainMs json.RawMessage `json:"pausedRemainMs"`
		Challenge      json.RawMessage `json:"challenge"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}

	if len(body.EventEndsAt) > 0 {
		_ = s.svc.Settings.SetRaw(model.KeyEventEndsAt, body.EventEndsAt)
	}
	if len(body.EventStartsAt) > 0 {
		_ = s.svc.Settings.SetRaw(model.KeyEventStartsAt, body.EventStartsAt)
	}
	if body.EventPaused != nil {
		_ = s.svc.Settings.Set(model.KeyEventPaused, *body.EventPaused)
	}
	if len(body.PausedRemainMs) > 0 {
		_ = s.svc.Settings.SetRaw(model.KeyPausedRemainMs, body.PausedRemainMs)
	}

	if len(body.Challenge) > 0 {
		var ch model.Challenge
		if err := json.Unmarshal(body.Challenge, &ch); err != nil {
			fail(c, apperr.InvalidInput("فرمت چالش نامعتبر است."))
			return
		}
		if ch.Image != nil && *ch.Image != "" && !challengeImageRe.MatchString(*ch.Image) {
			fail(c, apperr.InvalidInput("فرمت تصویر چالش نامعتبر است."))
			return
		}
		if r := []rune(ch.SubmitMessage); len(r) > 600 {
			ch.SubmitMessage = string(r[:600])
		}
		_ = s.svc.Settings.Set(model.KeyChallenge, ch)
	}

	c.JSON(http.StatusOK, s.svc.Settings.CurrentConfig())
}
