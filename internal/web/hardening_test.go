package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool-challenge/internal/config"
	"github.com/dotinschool/dotinschool-challenge/internal/database"
	"github.com/dotinschool/dotinschool-challenge/internal/model"
	"github.com/dotinschool/dotinschool-challenge/internal/service"
)

// hardeningRig is a router over its own in-memory database plus an admin token.
type hardeningRig struct {
	svc    *service.Services
	router *gin.Engine
	admin  string
}

func newHardeningRig(t *testing.T, dbName string) *hardeningRig {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	log := logrus.New()
	log.SetOutput(io.Discard)
	cfg := &config.Config{AdminUsername: "administrator", DataDir: t.TempDir()}
	svc := service.New(cfg, db, log)
	return &hardeningRig{
		svc:    svc,
		router: NewRouter(Options{Config: cfg, Services: svc, Log: log, Version: "test"}),
		admin:  svc.Sessions.IssueAdmin("administrator"),
	}
}

func (r *hardeningRig) do(method, path string, body any, bearer string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	return rec
}

func codeOf(rec *httptest.ResponseRecorder) string {
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Code
}

// The challenge must not be readable through the public API before the start,
// during a pause, or after the end — only while live, or by the admin.
func TestGetConfig_ChallengeHiddenUntilLive(t *testing.T) {
	r := newHardeningRig(t, "hardening_config")
	now := time.Now().UnixMilli()

	challengeOf := func(bearer string) json.RawMessage {
		rec := r.do(http.MethodGet, "/api/config", nil, bearer)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/config: status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body struct {
			Challenge json.RawMessage `json:"challenge"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return body.Challenge
	}
	hidden := func(raw json.RawMessage) bool { return len(raw) == 0 || string(raw) == "null" }

	states := []struct {
		name               string
		startsAt, endsAt   any
		paused, wantPublic bool
	}{
		{"scheduled", now + 3600000, now + 7200000, false, false},
		{"never started", nil, nil, false, false},
		{"paused", nil, now + 3600000, true, false},
		{"ended", nil, now - 1000, false, false},
		{"live", nil, now + 3600000, false, true},
	}
	for _, st := range states {
		_ = r.svc.Settings.Set(model.KeyEventStartsAt, st.startsAt)
		_ = r.svc.Settings.Set(model.KeyEventEndsAt, st.endsAt)
		_ = r.svc.Settings.Set(model.KeyEventPaused, st.paused)

		if got := challengeOf(""); hidden(got) == st.wantPublic {
			t.Errorf("%s: public challenge = %s, want visible=%v", st.name, got, st.wantPublic)
		}
		if got := challengeOf(r.admin); hidden(got) {
			t.Errorf("%s: the admin must always see the challenge", st.name)
		}
	}
}

// A saved AI key is never reused against a different base URL.
func TestPutAISettings_KeyBoundToBaseURL(t *testing.T) {
	r := newHardeningRig(t, "hardening_ai")
	put := func(baseURL, key string) *httptest.ResponseRecorder {
		return r.do(http.MethodPut, "/api/admin/ai-settings", map[string]any{
			"activeId":  "p1",
			"providers": []map[string]any{{"id": "p1", "label": "main", "baseUrl": baseURL, "model": "m", "apiKey": key}},
		}, r.admin)
	}

	if rec := put("https://api.example.com/v1", "sk-secret-key-123456"); rec.Code != http.StatusOK {
		t.Fatalf("initial save: status=%d body=%s", rec.Code, rec.Body.String())
	}
	// same endpoint, key left masked: kept
	if rec := put("https://api.example.com/v1/", ""); rec.Code != http.StatusOK {
		t.Fatalf("resave same endpoint: status=%d body=%s", rec.Code, rec.Body.String())
	}
	// new endpoint, key left masked: refused, nothing changed
	rec := put("https://attacker.example/v1", "")
	if rec.Code != http.StatusBadRequest || codeOf(rec) != "KEY_REQUIRED" {
		t.Fatalf("endpoint swap without key: status=%d code=%q", rec.Code, codeOf(rec))
	}
	if got := r.svc.AI.SettingsSnapshot().Providers[0].BaseURL; got != "https://api.example.com/v1" {
		t.Fatalf("base URL changed to %q after a refused save", got)
	}
	// new endpoint with a freshly typed key: allowed
	if rec := put("https://other.example/v1", "sk-another-key-654321"); rec.Code != http.StatusOK {
		t.Fatalf("endpoint swap with new key: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// The SMS key (panel or .env) is never sent to a newly configured endpoint
// unless it is typed again.
func TestPutSMSSettings_KeyBoundToEndpoint(t *testing.T) {
	r := newHardeningRig(t, "hardening_sms")
	put := func(baseURL, key string) *httptest.ResponseRecorder {
		return r.do(http.MethodPut, "/api/admin/sms-settings", map[string]any{"apiKey": key, "baseUrl": baseURL}, r.admin)
	}

	if rec := put("", "ippanel-secret-key"); rec.Code != http.StatusOK {
		t.Fatalf("initial save: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec := put("https://attacker.example/send", "")
	if rec.Code != http.StatusBadRequest || codeOf(rec) != "KEY_REQUIRED" {
		t.Fatalf("endpoint swap without key: status=%d code=%q", rec.Code, codeOf(rec))
	}
	if rec := put("https://attacker.example/send", "a-new-typed-key"); rec.Code != http.StatusOK {
		t.Fatalf("endpoint swap with new key: status=%d body=%s", rec.Code, rec.Body.String())
	}
	// unchanged endpoint, masked key: fine
	if rec := put("https://attacker.example/send", ""); rec.Code != http.StatusOK {
		t.Fatalf("resave same endpoint: status=%d body=%s", rec.Code, rec.Body.String())
	}
	// back to the default endpoint: no key needed
	if rec := put("", ""); rec.Code != http.StatusOK {
		t.Fatalf("reset to default endpoint: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
