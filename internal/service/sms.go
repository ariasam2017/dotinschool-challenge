package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/config"
	"github.com/dotinschool/dotinschool/internal/model"
)

// SMSProvider sends a pattern (template) SMS. The registration/login OTP and the
// "new registration" admin notification all go through this one method.
type SMSProvider interface {
	SendPattern(phone, patternCode string, params map[string]string) error
	// Verbose is used by the admin SMS diagnostic endpoint — it never hides the
	// upstream status/body so a misconfigured pattern, line, or credit is
	// actually visible.
	Verbose(phone, patternCode string, params map[string]string) SMSResult
	// Info reports the *currently effective* configuration (panel settings take
	// priority per-field over the .env fallback) — always fresh, never cached,
	// so a panel edit takes effect immediately with no restart.
	Info() SMSInfo
}

// SMSResult is the full outcome of one send attempt.
type SMSResult struct {
	OK       bool   `json:"ok"`
	Status   int    `json:"status"`
	Endpoint string `json:"endpoint"`
	Message  string `json:"message"`
	RawBody  string `json:"rawBody"`
}

// SMSInfo is the non-secret effective configuration, for the admin panel.
type SMSInfo struct {
	Configured      bool   `json:"configured"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Sender          string `json:"sender"`
	RegisterPattern string `json:"registerPattern"`
	LoginPattern    string `json:"loginPattern"`
	AdminPattern    string `json:"adminPattern"`
	AdminPhone      string `json:"adminPhone"`
	AuthScheme      string `json:"authScheme"`
	Source          string `json:"source"` // "panel" | "env" | ""
}

// ---- ippanel implementation -------------------------------------------

type ippanelProvider struct {
	cfg      *config.Config  // env fallback, per field
	settings *SettingService // panel override (DB + self-healing sidecar)
	client   *http.Client
	log      *logrus.Logger
}

func newIppanelProvider(cfg *config.Config, settings *SettingService, log *logrus.Logger) *ippanelProvider {
	return &ippanelProvider{
		cfg:      cfg,
		settings: settings,
		client:   &http.Client{Timeout: 12 * time.Second},
		log:      log,
	}
}

const defaultIppanelEndpoint = "https://edge.ippanel.com/v1/api/send"

// resolve merges the panel-managed settings over the .env defaults, field by
// field — so an admin can set *only* the API key from the panel and keep
// everything else (sender, patterns) at their working defaults.
func (p *ippanelProvider) resolve() SMSInfo {
	panel := p.settings.SMSSettings()
	pick := func(panelVal, envVal string) string {
		if strings.TrimSpace(panelVal) != "" {
			return panelVal
		}
		return envVal
	}
	source := "env"
	if strings.TrimSpace(panel.APIKey) != "" {
		source = "panel"
	}
	apiKey := pick(panel.APIKey, p.cfg.IppanelAPIKey)
	return SMSInfo{
		Configured:      apiKey != "",
		Provider:        "ippanel",
		Endpoint:        strings.TrimRight(pick(panel.BaseURL, orDefault(p.cfg.IppanelBaseURL, defaultIppanelEndpoint)), "/"),
		Sender:          pick(panel.Sender, p.cfg.IppanelSender),
		RegisterPattern: pick(panel.RegisterPattern, p.cfg.IppanelRegisterPattern),
		LoginPattern:    pick(panel.LoginPattern, p.cfg.IppanelLoginPattern),
		AdminPattern:    pick(panel.AdminNotifyPattern, p.cfg.IppanelAdminNotifyPattern),
		AdminPhone:      pick(panel.AdminPhone, p.cfg.IppanelAdminPhone),
		AuthScheme:      orDefault(pick(panel.AuthScheme, p.cfg.IppanelAuthScheme), "raw"),
		Source:          source,
	}
}

// apiKey resolves just the secret (kept separate from SMSInfo, which is safe
// to hand to the browser).
func (p *ippanelProvider) apiKey() string {
	panel := p.settings.SMSSettings()
	if strings.TrimSpace(panel.APIKey) != "" {
		return panel.APIKey
	}
	return p.cfg.IppanelAPIKey
}

type ippanelRequest struct {
	SendingType string            `json:"sending_type"`
	FromNumber  string            `json:"from_number"`
	Code        string            `json:"code"`
	Recipients  []string          `json:"recipients"`
	Params      map[string]string `json:"params"`
}

// ippanelResponse covers the shapes the various ippanel gateways return:
//
//	{"meta":{"status":true,"message":"..."}, "data":{...}}     (edge / web service)
//	{"status":"OK","code":200,"error_message":"...","data":{}} (api.ippanel.com v1)
type ippanelResponse struct {
	Meta struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	} `json:"meta"`
	Status       any    `json:"status"`
	ErrorMessage string `json:"error_message"`
	Message      string `json:"message"`
	Errors       any    `json:"errors"`
}

func (r ippanelResponse) succeeded(httpStatus int) bool {
	if httpStatus < 200 || httpStatus >= 300 {
		return false
	}
	if r.Meta.Status {
		return true
	}
	switch v := r.Status.(type) {
	case string:
		return strings.EqualFold(v, "OK")
	case bool:
		return v
	case float64:
		return v == 200 || v == 1
	}
	// 2xx with no recognisable failure marker → treat as success.
	return r.Meta.Message == "" && r.ErrorMessage == "" && r.Errors == nil
}

func (r ippanelResponse) message() string {
	for _, s := range []string{r.Meta.Message, r.ErrorMessage, r.Message} {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	if r.Errors != nil {
		if b, _ := json.Marshal(r.Errors); len(b) > 2 {
			return string(b)
		}
	}
	return ""
}

func authHeaderFor(scheme, key string) string {
	switch strings.ToLower(scheme) {
	case "accesskey":
		return "AccessKey " + key
	case "apikey":
		return "apikey " + key
	case "bearer":
		return "Bearer " + key
	default:
		return key
	}
}

func (p *ippanelProvider) Verbose(phone, patternCode string, params map[string]string) SMSResult {
	info := p.resolve()
	key := p.apiKey()
	res := SMSResult{Endpoint: info.Endpoint}
	if key == "" {
		res.Message = "سرویس پیامک تنظیم نشده — کلید ippanel را در پنل (نمای کلی → کارت «پیامک») یا در .env بگذارید."
		return res
	}

	body, _ := json.Marshal(ippanelRequest{
		SendingType: "pattern",
		FromNumber:  info.Sender,
		Code:        patternCode,
		Recipients:  []string{"+98" + strings.TrimLeft(phone, "0")},
		Params:      params,
	})

	req, err := http.NewRequest(http.MethodPost, info.Endpoint, bytes.NewReader(body))
	if err != nil {
		res.Message = err.Error()
		return res
	}
	req.Header.Set("Authorization", authHeaderFor(info.AuthScheme, key))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpRes, err := p.client.Do(req)
	if err != nil {
		res.Message = "اتصال به سرویس پیامک برقرار نشد: " + err.Error()
		p.log.WithError(err).WithField("endpoint", info.Endpoint).Error("ippanel: request failed")
		return res
	}
	defer httpRes.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(httpRes.Body, 1<<20))
	res.Status = httpRes.StatusCode
	res.RawBody = string(raw)

	var parsed ippanelResponse
	_ = json.Unmarshal(raw, &parsed)
	res.OK = parsed.succeeded(httpRes.StatusCode)
	res.Message = parsed.message()

	if !res.OK {
		p.log.WithFields(logrus.Fields{
			"endpoint": info.Endpoint,
			"status":   httpRes.StatusCode,
			"pattern":  patternCode,
			"body":     truncate(string(raw), 800),
		}).Error("ippanel: pattern send rejected")
	}
	return res
}

func (p *ippanelProvider) SendPattern(phone, patternCode string, params map[string]string) error {
	r := p.Verbose(phone, patternCode, params)
	if r.OK {
		return nil
	}
	if r.Message != "" {
		return fmt.Errorf("سرویس پیامک: %s", r.Message)
	}
	return fmt.Errorf("ارسال پیامک ناموفق بود (کد %d)", r.Status)
}

func (p *ippanelProvider) Info() SMSInfo { return p.resolve() }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// MaskedSMSSettings is the safe shape for GET/PUT /api/admin/sms-settings — the
// key is never sent back in full.
type MaskedSMSSettings struct {
	HasKey             bool   `json:"hasKey"`
	KeyMask            string `json:"keyMask"`
	Sender             string `json:"sender"`
	RegisterPattern    string `json:"registerPattern"`
	LoginPattern       string `json:"loginPattern"`
	AdminNotifyPattern string `json:"adminNotifyPattern"`
	AdminPhone         string `json:"adminPhone"`
	BaseURL            string `json:"baseUrl"`
	AuthScheme         string `json:"authScheme"`
}

// MaskedSMSSettingsFor strips the API key down to a display-safe mask.
func MaskedSMSSettingsFor(v model.SMSSettings) MaskedSMSSettings {
	return MaskedSMSSettings{
		HasKey:             v.APIKey != "",
		KeyMask:            MaskKey(v.APIKey),
		Sender:             v.Sender,
		RegisterPattern:    v.RegisterPattern,
		LoginPattern:       v.LoginPattern,
		AdminNotifyPattern: v.AdminNotifyPattern,
		AdminPhone:         v.AdminPhone,
		BaseURL:            v.BaseURL,
		AuthScheme:         v.AuthScheme,
	}
}
