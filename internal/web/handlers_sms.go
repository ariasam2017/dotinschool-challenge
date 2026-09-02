package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool/internal/apperr"
)

// GET /api/admin/sms-status — the non-secret SMS provider configuration, so the
// admin can see which endpoint / sender / pattern codes are actually in effect.
func (s *server) handleSMSStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.svc.SMS.Info())
}

// POST /api/admin/sms-test — send a real pattern SMS to a phone and return the
// full upstream response (status + body), so a rejected pattern / line / credit
// is visible instead of a generic failure.
func (s *server) handleSMSTest(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("sms-test", c.ClientIP(), 8, 10*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}

	var body struct {
		Phone   string `json:"phone"`
		Purpose string `json:"purpose"` // register | login
		Pattern string `json:"pattern"` // optional explicit pattern code
		Code    string `json:"code"`    // optional test value; default 12345
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}

	phone := strings.TrimSpace(body.Phone)
	phone = strings.TrimPrefix(phone, "+98")
	phone = strings.TrimPrefix(phone, "0")
	if !validPhone(phone) {
		fail(c, apperr.InvalidInput("شمارهٔ موبایل باید به شکل ۹XXXXXXXXX باشد (بدون صفر و بدون +۹۸)."))
		return
	}

	info := s.svc.SMS.Info()
	pattern := strings.TrimSpace(body.Pattern)
	if pattern == "" {
		if body.Purpose == "login" {
			pattern = info.LoginPattern
		} else {
			pattern = info.RegisterPattern
		}
	}
	if pattern == "" {
		fail(c, apperr.InvalidInput("کد پترن پیامک تنظیم نشده است."))
		return
	}

	code := strings.TrimSpace(body.Code)
	if code == "" {
		code = "12345"
	}

	result := s.svc.SMS.Verbose(phone, pattern, map[string]string{"code": code})
	result.Endpoint = info.Endpoint
	c.JSON(http.StatusOK, result)
}
