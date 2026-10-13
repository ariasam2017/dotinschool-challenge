package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/config"
	"github.com/dotinschool/dotinschool/internal/model"
)

// AI limits and defaults, identical to the original server.
const (
	AIDefaultBaseURL = "https://api.openai.com/v1"
	AIDefaultModel   = "gpt-4o-mini"
	AIMaxCodeChars   = 60000
	AIMaxProviders   = 20

	aiChatTimeout  = 90 * time.Second
	aiProbeTimeout = 8 * time.Second
)

// AIPreset is a ready-made row for the panel form; the admin can type anything.
type AIPreset struct {
	Label   string `json:"label"`
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
}

// AIPresets are shown as suggestions in the provider editor.
var AIPresets = []AIPreset{
	{"OpenAI (ChatGPT)", "https://api.openai.com/v1", "gpt-4o-mini"},
	{"GapGPT", "https://api.gapgpt.app/v1", "gpt-4o-mini"},
	{"Groq", "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile"},
	{"OpenRouter", "https://openrouter.ai/api/v1", "meta-llama/llama-3.1-8b-instruct:free"},
	{"DeepSeek", "https://api.deepseek.com/v1", "deepseek-chat"},
	{"Gemini (سازگار با OpenAI)", "https://generativelanguage.googleapis.com/v1beta/openai", "gemini-2.0-flash"},
}

// AIRuntimeConfig is the effective configuration for a single call.
type AIRuntimeConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Label   string
	Source  string // "panel" | "env"
}

// ChatMessage is one OpenAI chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResult is what a successful completion returns.
type ChatResult struct {
	Content string          `json:"content"`
	Model   string          `json:"model"`
	Usage   json.RawMessage `json:"usage"`
}

// ProbeResult is the outcome of a GET /models check. It never carries an error.
type ProbeResult struct {
	OK     bool
	Status int
	Reason string
}

// AIService talks to any OpenAI-compatible Chat Completions endpoint
// (OpenAI, GapGPT, Groq, OpenRouter, DeepSeek, Gemini-compat).
type AIService struct {
	cfg      *config.Config
	settings *SettingService
	log      *logrus.Logger

	chatClient  *http.Client
	probeClient *http.Client
}

func newAIService(cfg *config.Config, settings *SettingService, log *logrus.Logger) *AIService {
	return &AIService{
		cfg:         cfg,
		settings:    settings,
		log:         log,
		chatClient:  &http.Client{Timeout: aiChatTimeout},
		probeClient: &http.Client{Timeout: aiProbeTimeout},
	}
}

// SettingsSnapshot returns the current panel provider list (with self-healing
// applied), a convenience wrapper for handlers.
func (a *AIService) SettingsSnapshot() model.AISettings {
	return a.settings.AISettings()
}

// ResolveConfig returns the effective config: the panel's active provider
// first, then the .env fallback, otherwise nil.
func (a *AIService) ResolveConfig() *AIRuntimeConfig {
	s := a.settings.AISettings()

	var active *model.AIProvider
	for i := range s.Providers {
		if s.Providers[i].ID == s.ActiveID && s.Providers[i].APIKey != "" {
			active = &s.Providers[i]
			break
		}
	}
	if active == nil {
		for i := range s.Providers {
			if s.Providers[i].APIKey != "" {
				active = &s.Providers[i]
				break
			}
		}
	}
	if active != nil && active.APIKey != "" {
		return &AIRuntimeConfig{
			APIKey:  active.APIKey,
			BaseURL: strings.TrimRight(orDefault(active.BaseURL, AIDefaultBaseURL), "/"),
			Model:   a.ModelOrDefault(active.Model),
			Label:   orDefault(active.Label, "سرویس ذخیره‌شده در پنل"),
			Source:  "panel",
		}
	}

	if a.cfg.AIEnvAPIKey != "" {
		return &AIRuntimeConfig{
			APIKey:  a.cfg.AIEnvAPIKey,
			BaseURL: strings.TrimRight(orDefault(a.cfg.AIEnvBaseURL, AIDefaultBaseURL), "/"),
			Model:   a.ModelOrDefault(a.cfg.AIEnvModel),
			Label:   "تنظیمات فایل .env",
			Source:  "env",
		}
	}
	return nil
}

// ModelOrDefault is the model name to actually send upstream. An empty value —
// or one equal to the panel's own admin username, which browser password
// managers routinely autofill into the "نام مدل" text field that sits directly
// before the API-key field — falls back to the default model instead of being
// sent verbatim. (An OpenAI-compatible gateway rejects a model literally named
// "administrator" with "No available channel for model administrator".)
func (a *AIService) ModelOrDefault(m string) string {
	m = strings.TrimSpace(m)
	if m == "" {
		return AIDefaultModel
	}
	if a.cfg != nil && a.cfg.AdminUsername != "" && strings.EqualFold(m, a.cfg.AdminUsername) {
		return AIDefaultModel
	}
	return m
}

// Chat performs one completion. override wins over ResolveConfig when non-nil.
func (a *AIService) Chat(messages []ChatMessage, override *AIRuntimeConfig) (*ChatResult, error) {
	cfg := override
	if cfg == nil {
		cfg = a.ResolveConfig()
	}
	if cfg == nil || cfg.APIKey == "" {
		return nil, apperr.New(400, "AI_NOT_CONFIGURED",
			"هیچ سرویس هوش مصنوعی فعالی تنظیم نشده — از بخش «سرویس‌های هوش مصنوعی» در پنل، یک کلید ثبت و آن را فعال کنید.")
	}

	endpoint := strings.TrimRight(orDefault(cfg.BaseURL, AIDefaultBaseURL), "/") + "/chat/completions"
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, apperr.New(400, "AI_BAD_URL", "آدرس پایه سرویس نامعتبر است: "+cfg.BaseURL)
	}

	payload, _ := json.Marshal(map[string]any{
		"model":       orDefault(cfg.Model, AIDefaultModel),
		"messages":    messages,
		"temperature": 0.2,
		"stream":      false,
	})

	ctx, cancel := context.WithTimeout(context.Background(), aiChatTimeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := a.chatClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, apperr.New(504, "AI_TIMEOUT",
				"پاسخ سرویس هوش مصنوعی خیلی طول کشید — دوباره تلاش کنید یا مدل سبک‌تری انتخاب کنید.")
		}
		return nil, apperr.New(502, "AI_CONNECT_FAILED", "اتصال به سرویس هوش مصنوعی برقرار نشد: "+err.Error())
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))

	var parsed struct {
		Model   string          `json:"model"`
		Usage   json.RawMessage `json:"usage"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &parsed)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := parsed.Error.Message
		low := strings.ToLower(msg)
		if strings.Contains(low, "no available channel for model") ||
			strings.Contains(low, "model_not_found") ||
			strings.Contains(low, "model not found") ||
			strings.Contains(low, "no such model") ||
			strings.Contains(low, "invalid model") ||
			(strings.Contains(low, "model") && strings.Contains(low, "does not exist")) {
			return nil, apperr.New(502, "AI_MODEL_UNAVAILABLE",
				"مدل «"+orDefault(cfg.Model, AIDefaultModel)+"» روی این سرویس در دسترس نیست — «نام مدل» را در تنظیمات بررسی کنید (مثلا gpt-4o-mini).")
		}
		if msg == "" {
			msg = fmt.Sprintf("سرویس هوش مصنوعی خطا داد (کد %d).", res.StatusCode)
		}
		return nil, apperr.New(502, "AI_UPSTREAM_ERROR", msg)
	}

	var content string
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
		if content == "" {
			content = parsed.Choices[0].Text
		}
	}
	if strings.TrimSpace(content) == "" {
		return nil, apperr.New(502, "AI_EMPTY_RESPONSE", "پاسخ سرویس هوش مصنوعی قابل خواندن نبود.")
	}

	return &ChatResult{
		Content: strings.TrimSpace(content),
		Model:   orDefault(parsed.Model, orDefault(cfg.Model, AIDefaultModel)),
		Usage:   parsed.Usage,
	}, nil
}

// Probe does a lightweight GET {baseURL}/models with the given key, to tell
// which service a generic "sk-..." key actually belongs to. Never errors.
func (a *AIService) Probe(baseURL, apiKey string) ProbeResult {
	endpoint := strings.TrimRight(baseURL, "/") + "/models"
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return ProbeResult{Reason: "bad-url"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), aiProbeTimeout)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := a.probeClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ProbeResult{Reason: "timeout"}
		}
		return ProbeResult{Reason: "unreachable"}
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
	return ProbeResult{OK: res.StatusCode >= 200 && res.StatusCode < 300, Status: res.StatusCode}
}

// MaskKey returns a display-safe rendering of an API key.
func MaskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "••••••••"
	}
	return "••••••" + k[len(k)-4:]
}

// LooksLikeMask reports whether v is a mask we handed the browser earlier (so
// the caller should keep the stored key rather than overwrite it with dots).
func LooksLikeMask(v string) bool {
	return strings.HasPrefix(v, "•")
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
