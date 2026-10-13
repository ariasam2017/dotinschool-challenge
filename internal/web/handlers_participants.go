package web

import (
	"bytes"
	"encoding/json"
	"math"
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
			fail(c, apperr.Forbidden("اجازه ویرایش این شرکت‌کننده را ندارید."))
			return
		}
	}

	touchesProgress := body.Status != nil || body.Code != nil ||
		len(body.Score) > 0 || len(body.ExecMs) > 0 || len(body.ExecDepth) > 0

	// One-time submission: once a participant has submitted, they cannot
	// submit again or otherwise touch their challenge progress (status, code,
	// score, exec stats) — resubmitting no longer overwrites the graded
	// result. The admin is exempt, so grading/correction still works.
	if !isAdmin && p.Status == model.StatusSubmitted && touchesProgress {
		fail(c, apperr.New(409, "ALREADY_SUBMITTED",
			"شما قبلا کد خود را ارسال کرده‌اید — امکان ارسال یا ویرایش دوباره وجود ندارد."))
		return
	}

	// status is an allow-list: a participant may only move forward to
	// started/submitted; the admin may also reset to registered.
	if body.Status != nil {
		switch *body.Status {
		case model.StatusStarted, model.StatusSubmitted:
		case model.StatusRegistered:
			if !isAdmin {
				fail(c, apperr.Forbidden("فقط ادمین می‌تواند وضعیت را بازنشانی کند."))
				return
			}
		default:
			fail(c, apperr.InvalidInput("وضعیت نامعتبر است."))
			return
		}
	}

	// the AI verdict is the admin's call, never the participant's
	if body.AIFlag != nil {
		if !isAdmin {
			fail(c, apperr.Forbidden("فقط ادمین می‌تواند این مقدار را تغییر دهد."))
			return
		}
		if f := *body.AIFlag; f != "" && f != model.AIFlagHuman && f != model.AIFlagSuspicious {
			fail(c, apperr.InvalidInput("مقدار برچسب هوش مصنوعی نامعتبر است."))
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

	// live-event lock: started/submitted transitions — and, for a participant,
	// any change to code/score/exec stats — only while the event is live, so a
	// quiet code or score patch after the clock runs out is refused too.
	statusMove := body.Status != nil && (*body.Status == model.StatusSubmitted || *body.Status == model.StatusStarted)
	if statusMove || (!isAdmin && touchesProgress) {
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
	applyNullableInt(body.Score, &p.Score, 0, 100)
	applyNullableInt(body.ExecMs, &p.ExecMs, 0, maxExecStat)
	applyNullableInt(body.ExecDepth, &p.ExecDepth, 0, maxExecStat)
	applyNullableInt(body.AIScore, &p.AIScore, 0, 100)

	if err := s.svc.Participants.Save(p); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// maxExecStat bounds the reported execution time/depth — a client-sent number
// must never be negative or absurd enough to wreck the admin's sorting.
const maxExecStat = 10_000_000

// applyNullableInt mirrors `next.x = body.x !== undefined ? body.x : row.x`.
// raw nil  -> field untouched
// raw null -> field set to nil
// raw N    -> field set to N, clamped to lo..hi
func applyNullableInt(raw json.RawMessage, dst **int, lo, hi int) {
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
	n = math.Max(float64(lo), math.Min(float64(hi), n))
	v := int(n)
	*dst = &v
}
