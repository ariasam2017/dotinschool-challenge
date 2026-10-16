package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool-challenge/internal/config"
	"github.com/dotinschool/dotinschool-challenge/internal/database"
	"github.com/dotinschool/dotinschool-challenge/internal/model"
	"github.com/dotinschool/dotinschool-challenge/internal/service"
)

// Once a participant has submitted, resubmitting (or quietly patching the code
// without a status field) must be rejected and must never touch the stored
// result — while the admin stays exempt for grading/correction.
func TestPatchParticipant_OneTimeSubmission(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	log := logrus.New()
	log.SetOutput(io.Discard) // keep `go test -v` output readable
	cfg := &config.Config{AdminUsername: "administrator"}
	svc := service.New(cfg, db, log)

	// make the event live, so the status transition itself is allowed
	now := time.Now().UnixMilli()
	_ = svc.Settings.Set(model.KeyEventEndsAt, now+3600000)
	_ = svc.Settings.Set(model.KeyEventStartsAt, nil)
	_ = svc.Settings.Set(model.KeyEventPaused, false)

	p, err := svc.Participants.Create("Test", "User", "9120000000")
	if err != nil {
		t.Fatalf("create participant: %v", err)
	}
	tok := svc.Sessions.IssueParticipant(p.ID)

	router := NewRouter(Options{Config: cfg, Services: svc, Log: log, Version: "test"})

	patch := func(body map[string]any, bearer string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/participants/"+p.ID, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	errCode := func(rec *httptest.ResponseRecorder) string {
		var body struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return body.Code
	}

	// a participant can't stamp their own AI verdict, invent a status, or
	// reset themselves back to registered
	for _, bad := range []map[string]any{
		{"sessionToken": tok, "aiFlag": "human"},
		{"sessionToken": tok, "status": "winner"},
		{"sessionToken": tok, "status": "registered"},
	} {
		if rec := patch(bad, ""); rec.Code < 400 {
			t.Fatalf("forged patch %v accepted: status=%d body=%s", bad, rec.Code, rec.Body.String())
		}
	}

	// a quiet code patch is refused once the event clock has run out
	_ = svc.Settings.Set(model.KeyEventEndsAt, now-1000)
	if rec := patch(map[string]any{"sessionToken": tok, "code": "late"}, ""); rec.Code != 423 {
		t.Fatalf("code patch after the event ended: status=%d body=%s", rec.Code, rec.Body.String())
	}
	_ = svc.Settings.Set(model.KeyEventEndsAt, now+3600000)

	// first submission succeeds, with the client-sent score clamped to 0..100
	rec := patch(map[string]any{"sessionToken": tok, "status": "submitted", "code": "first", "score": 80, "execMs": -5}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("first submit: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// resubmitting is rejected ...
	rec = patch(map[string]any{"sessionToken": tok, "status": "submitted", "code": "second", "score": 5}, "")
	if rec.Code != http.StatusConflict || errCode(rec) != "ALREADY_SUBMITTED" {
		t.Fatalf("resubmit: status=%d code=%q body=%s", rec.Code, errCode(rec), rec.Body.String())
	}

	// ... and so is a quiet code-only patch (no status field) after submission
	rec = patch(map[string]any{"sessionToken": tok, "code": "sneaky"}, "")
	if rec.Code != http.StatusConflict || errCode(rec) != "ALREADY_SUBMITTED" {
		t.Fatalf("code-only patch after submit: status=%d code=%q body=%s", rec.Code, errCode(rec), rec.Body.String())
	}

	// neither attempt touched the stored result
	got, err := svc.Participants.GetByID(p.ID)
	if err != nil {
		t.Fatalf("reload participant: %v", err)
	}
	if got.Code == nil || *got.Code != "first" {
		t.Fatalf("stored code = %v, want \"first\" (must not be overwritten)", got.Code)
	}
	if got.Score == nil || *got.Score != 80 {
		t.Fatalf("stored score = %v, want 80", got.Score)
	}
	if got.ExecMs == nil || *got.ExecMs != 0 {
		t.Fatalf("stored execMs = %v, want 0 (negative clamped)", got.ExecMs)
	}
	if got.AIFlag != nil {
		t.Fatalf("stored aiFlag = %v, want nil (participant may not set it)", *got.AIFlag)
	}

	// the admin is exempt from the one-time lock (grading/correction)
	adminTok := svc.Sessions.IssueAdmin("administrator")
	rec = patch(map[string]any{"score": 95}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin edit after submission: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
