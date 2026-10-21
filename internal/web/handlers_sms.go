package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool-challenge/internal/apperr"
	"github.com/dotinschool/dotinschool-challenge/internal/model"
	"github.com/dotinschool/dotinschool-challenge/internal/service"
)

// GET /api/admin/sms-status — the non-secret SMS provider configuration, so the
// admin can see which endpoint / sender / pattern codes are actually in effect.
func (s *server) handleSMSStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.svc.SMS.Info())
}

// GET /api/admin/sms-settings — the panel-managed ippanel config (masked key),
// so the admin can see what's saved and edit it — e.g. after moving the app to
// a new server, just paste the API key here instead of touching .env.
func (s *server) handleGetSMSSettings(c *gin.Context) {
	c.JSON(http.StatusOK, service.MaskedSMSSettingsFor(s.svc.Settings.SMSSettings()))
}

// PUT /api/admin/sms-settings — save the ippanel config. Leaving a field blank
// keeps the .env fallback for that field; the API key is only replaced when the
// admin actually types a new value (not the mask).
func (s *server) handlePutSMSSettings(c *gin.Context) {
	var body struct {
		APIKey             string `json:"apiKey"`
		Sender             string `json:"sender"`
		RegisterPattern    string `json:"registerPattern"`
		LoginPattern       string `json:"loginPattern"`
		AdminNotifyPattern string `json:"adminNotifyPattern"`
		AdminPhone         string `json:"adminPhone"`
		BaseURL            string `json:"baseUrl"`
		AuthScheme         string `json:"authScheme"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}

	baseURL := clip(strings.TrimRight(strings.TrimSpace(body.BaseURL), "/"), 300)
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			fail(c, apperr.InvalidInput("آدرس اندپوینت نامعتبر است."))
			return
		}
	}

	cur := s.svc.Settings.SMSSettings()
	typedKey := strings.TrimSpace(body.APIKey)
	newKey := typedKey != "" && !service.LooksLikeMask(typedKey)
	apiKey := cur.APIKey
	if newKey {
		apiKey = clip(typedKey, 400)
	}

	// The saved (or .env) key only ever goes to the endpoint it was set up
	// for. Pointing the panel at a new custom endpoint without re-entering the
	// key would let anyone holding an admin session receive it with the next
	// SMS. Clearing back to the default endpoint needs no key.
	if !newKey && baseURL != "" && !strings.EqualFold(baseURL, strings.TrimRight(cur.BaseURL, "/")) &&
		s.svc.SMS.Info().Configured {
		fail(c, apperr.New(400, "KEY_REQUIRED", "آدرس اندپوینت تغییر کرده — برای امنیت، کلید API را دوباره وارد کنید."))
		return
	}

	v := model.SMSSettings{
		APIKey:             apiKey,
		Sender:             clip(strings.TrimSpace(body.Sender), 40),
		RegisterPattern:    clip(strings.TrimSpace(body.RegisterPattern), 60),
		LoginPattern:       clip(strings.TrimSpace(body.LoginPattern), 60),
		AdminNotifyPattern: clip(strings.TrimSpace(body.AdminNotifyPattern), 60),
		AdminPhone:         clip(strings.TrimPrefix(strings.TrimSpace(body.AdminPhone), "0"), 15),
		BaseURL:            baseURL,
		AuthScheme:         clip(strings.TrimSpace(body.AuthScheme), 20),
	}
	if err := s.svc.Settings.SaveSMSSettings(v); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, service.MaskedSMSSettingsFor(v))
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
		fail(c, apperr.InvalidInput("شماره موبایل باید به شکل ۹XXXXXXXXX باشد (بدون صفر و بدون +۹۸)."))
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
