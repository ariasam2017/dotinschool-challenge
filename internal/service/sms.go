package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/config"
)

// SMSProvider sends a pattern (template) SMS. The registration/login OTP and the
// "new registration" admin notification all go through this one method.
type SMSProvider interface {
	SendPattern(phone, patternCode string, params map[string]string) error
}

// ---- ippanel implementation -------------------------------------------

type ippanelProvider struct {
	apiKey string
	sender string
	client *http.Client
	log    *logrus.Logger
}

func newIppanelProvider(cfg *config.Config, log *logrus.Logger) *ippanelProvider {
	return &ippanelProvider{
		apiKey: cfg.IppanelAPIKey,
		sender: cfg.IppanelSender,
		client: &http.Client{Timeout: 8 * time.Second},
		log:    log,
	}
}

type ippanelRequest struct {
	SendingType string            `json:"sending_type"`
	FromNumber  string            `json:"from_number"`
	Code        string            `json:"code"`
	Recipients  []string          `json:"recipients"`
	Params      map[string]string `json:"params"`
}

type ippanelResponse struct {
	Meta struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	} `json:"meta"`
}

func (p *ippanelProvider) SendPattern(phone, patternCode string, params map[string]string) error {
	if p.apiKey == "" {
		return errors.New("IPPANEL_API_KEY تنظیم نشده است.")
	}
	body, err := json.Marshal(ippanelRequest{
		SendingType: "pattern",
		FromNumber:  p.sender,
		Code:        patternCode,
		Recipients:  []string{"+98" + phone},
		Params:      params,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, "https://edge.ippanel.com/v1/api/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("اتصال به سرویس پیامک با تأخیر مواجه شد: %w", err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var parsed ippanelResponse
	_ = json.Unmarshal(raw, &parsed)

	if res.StatusCode >= 200 && res.StatusCode < 300 && parsed.Meta.Status {
		return nil
	}
	if parsed.Meta.Message != "" {
		return errors.New(parsed.Meta.Message)
	}
	return errors.New("ارسال پیامک ناموفق بود.")
}

// ---- disabled provider (no API key) ----------------------------------

type nullSMSProvider struct{}

func (nullSMSProvider) SendPattern(string, string, map[string]string) error {
	return errors.New("IPPANEL_API_KEY تنظیم نشده است.")
}
