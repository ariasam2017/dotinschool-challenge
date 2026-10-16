package web

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool-challenge/internal/model"
)

// GET /api/admin/appearance — the current draft + published appearance.
func (s *server) handleGetAppearance(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"draft":     s.svc.Settings.AppearanceDraftRaw(),
		"published": s.svc.Settings.AppearancePublishedRaw(),
	})
}

// PUT /api/admin/appearance — save the draft (validated). Does NOT publish.
func (s *server) handlePutAppearanceDraft(c *gin.Context) {
	var a model.Appearance
	if err := bindJSON(c, &a); err != nil {
		fail(c, err)
		return
	}
	if err := s.svc.Settings.SaveAppearanceDraft(&a); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"draft": mustRaw(a)})
}

// POST /api/admin/appearance/publish — make the draft live for every visitor.
func (s *server) handlePublishAppearance(c *gin.Context) {
	if err := s.svc.Settings.PublishAppearance(); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"published": s.svc.Settings.AppearancePublishedRaw()})
}

// POST /api/admin/appearance/revert — drop draft edits, back to published.
func (s *server) handleRevertAppearance(c *gin.Context) {
	if err := s.svc.Settings.RevertAppearanceDraft(); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"draft": s.svc.Settings.AppearanceDraftRaw()})
}

// POST /api/admin/appearance/reset — clear everything, back to the built-in design.
func (s *server) handleResetAppearance(c *gin.Context) {
	if err := s.svc.Settings.ResetAppearance(); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"draft":     s.svc.Settings.AppearanceDraftRaw(),
		"published": s.svc.Settings.AppearancePublishedRaw(),
	})
}

func mustRaw(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
