package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
)

// POST /api/client-errors — public, rate-limited. An entrant's browser reports a
// failed submission here. Identity is confirmed from the session token when
// present; this endpoint never hard-fails.
func (s *server) handleReportClientError(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("client-error", c.ClientIP(), 30, 10*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}

	var body struct {
		SessionToken  string `json:"sessionToken"`
		ParticipantID string `json:"participantId"`
		Name          string `json:"name"`
		Context       string `json:"context"`
		ErrorCode     string `json:"errorCode"`
		ErrorMessage  string `json:"errorMessage"`
		Code          string `json:"code"`
	}
	_ = c.ShouldBindJSON(&body) // tolerate a malformed/empty body

	var participantID, publicID, name string
	if id, ok := s.svc.Sessions.ResolveParticipant(body.SessionToken); ok {
		if p, err := s.svc.Participants.GetByID(id); err == nil {
			participantID, publicID, name = p.ID, p.PublicID, p.FirstName+" "+p.LastName
		}
	} else if body.ParticipantID != "" {
		if p, err := s.svc.Participants.GetByID(body.ParticipantID); err == nil {
			participantID, publicID, name = p.ID, p.PublicID, p.FirstName+" "+p.LastName
		}
	}
	if name == "" && strings.TrimSpace(body.Name) != "" {
		name = clip(strings.TrimSpace(body.Name), 120)
	}
	if name == "" {
		name = "ناشناس"
	}

	entry := model.ClientError{
		ParticipantID: participantID,
		PublicID:      publicID,
		Name:          name,
		Context:       orString(clip(body.Context, 40), "submit"),
		ErrorCode:     orString(clip(body.ErrorCode, 60), "UNKNOWN"),
		ErrorMessage:  clip(body.ErrorMessage, model.MaxClientErrors),
		CodeExcerpt:   clip(body.Code, 6000),
		UserAgent:     clip(c.GetHeader("User-Agent"), 300),
	}
	if err := s.svc.ClientErrors.Record(entry); err != nil {
		s.log.WithError(err).Warn("could not persist a client error report")
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/admin/client-errors — the reports table + unseen count.
func (s *server) handleListClientErrors(c *gin.Context) {
	list, err := s.svc.ClientErrors.List()
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// POST /api/admin/client-errors/seen — clear the unseen badge.
func (s *server) handleSeenClientErrors(c *gin.Context) {
	if err := s.svc.ClientErrors.MarkAllSeen(); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/admin/client-errors/:id — remove one report.
func (s *server) handleDeleteClientError(c *gin.Context) {
	deleted, err := s.svc.ClientErrors.DeleteOne(c.Param("id"))
	if err != nil {
		s.failWith(c, err)
		return
	}
	if !deleted {
		fail(c, apperr.NotFound("مورد یافت نشد."))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/admin/client-errors — clear them all.
func (s *server) handleClearClientErrors(c *gin.Context) {
	if err := s.svc.ClientErrors.DeleteAll(); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func orString(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
