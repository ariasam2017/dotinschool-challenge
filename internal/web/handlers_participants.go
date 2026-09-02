package web

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
)

// GET /api/participants — admin. Full list ordered by registration time.
func (s *server) handleListParticipants(c *gin.Context) {
	list, err := s.svc.Participants.List()
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// DELETE /api/participants/:id — admin.
func (s *server) handleDeleteParticipant(c *gin.Context) {
	deleted, err := s.svc.Participants.Delete(c.Param("id"))
	if err != nil {
		s.failWith(c, err)
		return
	}
	if !deleted {
		fail(c, apperr.NotFound("شرکت‌کننده پیدا نشد."))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// maxSubmittedCode caps the stored solution size (~5000 lines of Java).
const maxSubmittedCode = 200_000

type participantPatch struct {
	SessionToken *string `json:"sessionToken"`
	FirstName    *string `json:"firstName"`
	LastName     *string `json:"lastName"`
	Status       *string `json:"status"`
	Code         *string `json:"code"`
	AIFlag       *string `json:"aiFlag"`

	// number | null | absent — RawMessage keeps all three distinguishable.
	Score     json.RawMessage `json:"score"`
	ExecMs    json.RawMessage `json:"execMs"`
	ExecDepth json.RawMessage `json:"execDepth"`
	AIScore   json.RawMessage `json:"aiScore"`
}

// PATCH /api/participants/:id — dual-authenticated.
//
// Either the caller is the admin (may change any field) or holds the session
// token of *this* participant (may only progress their own challenge). Changing
// the name and aiScore is admin-only. started/submitted transitions are only
// accepted while the event is genuinely live (not a future schedule, not
// paused, not finished) — a server-side lock the client cannot bypass.
func (s *server) handlePatchParticipant(c *gin.Context) {
	id := c.Param("id")

	p, err := s.svc.Participants.GetByID(id)
	if err != nil {
		if s.notFound(err) {
			fail(c, apperr.NotFound("شرکت‌کننده پیدا نشد."))
			return
		}
		s.failWith(c, err)
		return
	}

	var body participantPatch
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}

	_, isAdmin := s.svc.Sessions.ResolveAdmin(bearerToken(c))
	if !isAdmin {
		token := ""
		if body.SessionToken != nil {
			token = *body.SessionToken
		}
		ownerID, aerr := s.resolveParticipant(token)
		if aerr != nil {
			fail(c, aerr)
			return
		}
		if ownerID != id {
			fail(c, apperr.Forbidden("اجازه‌ی ویرایش این شرکت‌کننده را ندارید."))
			return
		}
	}

	// name changes
	if body.FirstName != nil || body.LastName != nil {
		if !isAdmin {
			fail(c, apperr.Forbidden("فقط ادمین می‌تواند نام را تغییر دهد."))
			return
		}
		if body.FirstName != nil && (*body.FirstName == "" || len([]rune(*body.FirstName)) > 60) {
			fail(c, apperr.InvalidInput("نام نامعتبر است."))
			return
		}
		if body.LastName != nil && (*body.LastName == "" || len([]rune(*body.LastName)) > 60) {
			fail(c, apperr.InvalidInput("نام خانوادگی نامعتبر است."))
			return
		}
	}

	// live-event lock for started/submitted
	if body.Status != nil && (*body.Status == model.StatusSubmitted || *body.Status == model.StatusStarted) {
		now := nowMs()
		if startsAt := s.svc.Settings.EventStartsAt(); startsAt != nil && *startsAt > now {
			fail(c, apperr.New(423, "EVENT_NOT_STARTED", "چالش هنوز طبق زمان‌بندی شروع نشده است."))
			return
		}
		if s.svc.Settings.EventPaused() {
			fail(c, apperr.New(423, "EVENT_LOCKED", "رویداد توسط مدیر متوقف شده است."))
			return
		}
		if endsAt := s.svc.Settings.EventEndsAt(); endsAt != nil && *endsAt <= now {
			fail(c, apperr.New(423, "EVENT_LOCKED", "زمان رویداد به پایان رسیده است."))
			return
		}
	}

	// aiScore is admin-only and clamped 0..100
	if len(body.AIScore) > 0 && !isAdmin {
		fail(c, apperr.Forbidden("فقط ادمین می‌تواند این مقدار را تغییر دهد."))
		return
	}

	// ---- apply the allow-listed fields ----
	if body.Status != nil {
		p.Status = *body.Status
	}
	if body.Code != nil {
		// A challenge solution is never this big; clamp so a single row can't
		// bloat the DB or the admin's code-preview render.
		code := *body.Code
		if len(code) > maxSubmittedCode {
			code = code[:maxSubmittedCode]
		}
		p.Code = &code
	}
	if body.AIFlag != nil {
		p.AIFlag = body.AIFlag
	}
	if body.FirstName != nil {
		p.FirstName = *body.FirstName
	}
	if body.LastName != nil {
		p.LastName = *body.LastName
	}
	applyNullableInt(body.Score, &p.Score, false)
	applyNullableInt(body.ExecMs, &p.ExecMs, false)
	applyNullableInt(body.ExecDepth, &p.ExecDepth, false)
	applyNullableInt(body.AIScore, &p.AIScore, true)

	if err := s.svc.Participants.Save(p); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// applyNullableInt mirrors `next.x = body.x !== undefined ? body.x : row.x`.
// raw nil  -> field untouched
// raw null -> field set to nil
// raw N    -> field set to N (clamped to 0..100 when clamp is true, for aiScore)
func applyNullableInt(raw json.RawMessage, dst **int, clamp bool) {
	if len(raw) == 0 {
		return
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		*dst = nil
		return
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err != nil {
		return
	}
	v := int(n)
	if clamp {
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
	}
	*dst = &v
}
