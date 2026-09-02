package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/config"
)

// SMSProvider sends a pattern (template) SMS. The registration/login OTP and the
// "new registration" admin notification all go through this one method.
type SMSProvider interface {
	SendPattern(phone, patternCode string, params map[string]string) error
	// Verbose is used by the admin SMS diagnostic endpoint — it never hides the
	// upstream status/body so a misconfigured pattern, line, or credit is
	// actually visible.
	Verbose(phone, patternCode string, params map[string]string) SMSResult
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

// SMSInfo is the non-secret provider configuration, for the admin panel.
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
}

// ---- ippanel implementation -------------------------------------------

type ippanelProvider struct {
	cfg      *config.Config
	apiKey   string
	sender   string
	endpoint string
	// authScheme: "" (raw key), "AccessKey", "apikey", "Bearer"
	authScheme string
	client     *http.Client
	log        *logrus.Logger
}

func newIppanelProvider(cfg *config.Config, log *logrus.Logger) *ippanelProvider {
	endpoint := cfg.IppanelBaseURL
	if endpoint == "" {
		endpoint = "https://edge.ippanel.com/v1/api/send"
	}
	return &ippanelProvider{
		cfg:        cfg,
		apiKey:     cfg.IppanelAPIKey,
		sender:     cfg.IppanelSender,
		endpoint:   endpoint,
		authScheme: cfg.IppanelAuthScheme,
		client:     &http.Client{Timeout: 12 * time.Second},
		log:        log,
	}
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

func (p *ippanelProvider) authHeader() string {
	switch strings.ToLower(p.authScheme) {
	case "accesskey":
		return "AccessKey " + p.apiKey
	case "apikey":
		return "apikey " + p.apiKey
	case "bearer":
		return "Bearer " + p.apiKey
	default:
		return p.apiKey
	}
}

func (p *ippanelProvider) Verbose(phone, patternCode string, params map[string]string) SMSResult {
	res := SMSResult{Endpoint: p.endpoint}
	if p.apiKey == "" {
		res.Message = "IPPANEL_API_KEY تنظیم نشده است."
		return res
	}

	body, _ := json.Marshal(ippanelRequest{
		SendingType: "pattern",
		FromNumber:  p.sender,
		Code:        patternCode,
		Recipients:  []string{"+98" + strings.TrimLeft(phone, "0")},
		Params:      params,
	})

	req, err := http.NewRequest(http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		res.Message = err.Error()
		return res
	}
	req.Header.Set("Authorization", p.authHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpRes, err := p.client.Do(req)
	if err != nil {
		res.Message = "اتصال به سرویس پیامک برقرار نشد: " + err.Error()
		p.log.WithError(err).WithField("endpoint", p.endpoint).Error("ippanel: request failed")
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
			"endpoint": p.endpoint,
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
	return fmt.Errorf("ارسال پیامک ناموفق بود (کد %d).", r.Status)
}

func (p *ippanelProvider) Info() SMSInfo {
	scheme := p.authScheme
	if scheme == "" {
		scheme = "raw"
	}
	return SMSInfo{
		Configured:      p.apiKey != "",
		Provider:        "ippanel",
		Endpoint:        p.endpoint,
		Sender:          p.sender,
		RegisterPattern: p.cfg.IppanelRegisterPattern,
		LoginPattern:    p.cfg.IppanelLoginPattern,
		AdminPattern:    p.cfg.IppanelAdminNotifyPattern,
		AdminPhone:      p.cfg.IppanelAdminPhone,
		AuthScheme:      scheme,
	}
}

// ---- disabled provider (no API key) ----------------------------------

type nullSMSProvider struct{ cfg *config.Config }

func (nullSMSProvider) SendPattern(string, string, map[string]string) error {
	return errors.New("سرویس پیامک تنظیم نشده — کلید IPPANEL_API_KEY را در .env بگذارید.")
}
func (nullSMSProvider) Verbose(string, string, map[string]string) SMSResult {
	return SMSResult{Message: "سرویس پیامک تنظیم نشده — کلید IPPANEL_API_KEY را در .env بگذارید."}
}
func (n nullSMSProvider) Info() SMSInfo {
	i := SMSInfo{Provider: "ippanel", Endpoint: "https://edge.ippanel.com/v1/api/send", AuthScheme: "raw"}
	if n.cfg != nil {
		i.Sender = n.cfg.IppanelSender
		i.RegisterPattern = n.cfg.IppanelRegisterPattern
		i.LoginPattern = n.cfg.IppanelLoginPattern
		i.AdminPattern = n.cfg.IppanelAdminNotifyPattern
		i.AdminPhone = n.cfg.IppanelAdminPhone
	}
	return i
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
