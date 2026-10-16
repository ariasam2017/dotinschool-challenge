package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool-challenge/internal/apperr"
	"github.com/dotinschool/dotinschool-challenge/internal/model"
)

// The portable "move the entrants to another server" file. Unlike the whole-DB
// .db backup, this carries only the participants table and is imported row by
// row through GORM into the destination's own already-migrated schema — so it
// can't crash-loop a differently-built instance and needs no restart.
const (
	participantsFileKind    = "dotinschool.participants"
	participantsFileVersion = 1
)

type participantsFile struct {
	Kind         string              `json:"kind"`
	Version      int                 `json:"version"`
	ExportedAt   int64               `json:"exportedAt"`
	Count        int                 `json:"count"`
	Participants []model.Participant `json:"participants"`
}

// GET /api/admin/participants/export — every participant row, as a downloadable
// JSON file.
func (s *server) handleExportParticipants(c *gin.Context) {
	list, err := s.svc.Participants.List()
	if err != nil {
		s.failWith(c, err)
		return
	}
	out := participantsFile{
		Kind:         participantsFileKind,
		Version:      participantsFileVersion,
		ExportedAt:   nowMs(),
		Count:        len(list),
		Participants: list,
	}
	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		s.failWith(c, err)
		return
	}
	filename := "dotinschool-challenge-participants-" + time.Now().Format("2006-01-02") + ".json"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// POST /api/admin/participants/import — replace-all. The destination's
// participants table is wiped and refilled from the file. Validates the whole
// payload first; one bad row rejects the file and nothing is written.
func (s *server) handleImportParticipants(c *gin.Context) {
	var body participantsFile
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	if body.Kind != participantsFileKind || body.Version != participantsFileVersion {
		fail(c, apperr.New(400, "INVALID_FILE", "فایل انتقال شرکت‌کنندگان معتبر نیست."))
		return
	}
	if len(body.Participants) == 0 {
		fail(c, apperr.InvalidInput("فایل هیچ شرکت‌کننده‌ای ندارد."))
		return
	}

	seenID := make(map[string]bool, len(body.Participants))
	seenPhone := make(map[string]bool, len(body.Participants))
	seenPublic := make(map[string]bool, len(body.Participants))

	for i := range body.Participants {
		p := &body.Participants[i]
		where := fmt.Sprintf("شرکت‌کننده ردیف %d", i+1)

		if p.ID == "" {
			fail(c, apperr.InvalidInput(where+": شناسه (id) خالی است."))
			return
		}
		if p.PublicID == "" {
			fail(c, apperr.InvalidInput(where+": شناسه نمایشی (publicId) خالی است."))
			return
		}
		if p.FirstName == "" || len([]rune(p.FirstName)) > 60 ||
			p.LastName == "" || len([]rune(p.LastName)) > 60 {
			fail(c, apperr.InvalidInput(where+": نام یا نام خانوادگی نامعتبر است."))
			return
		}
		if !validPhone(p.Phone) {
			fail(c, apperr.InvalidInput(where+": شماره موبایل نامعتبر است."))
			return
		}
		switch p.Status {
		case model.StatusRegistered, model.StatusStarted, model.StatusSubmitted:
		default:
			fail(c, apperr.InvalidInput(where+": وضعیت نامعتبر است."))
			return
		}
		if p.RegisteredAt <= 0 {
			fail(c, apperr.InvalidInput(where+": زمان ثبت‌نام نامعتبر است."))
			return
		}
		if seenID[p.ID] || seenPhone[p.Phone] || seenPublic[p.PublicID] {
			fail(c, apperr.InvalidInput(where+": شناسه/موبایل/شناسه نمایشی تکراری در فایل."))
			return
		}
		seenID[p.ID], seenPhone[p.Phone], seenPublic[p.PublicID] = true, true, true

		if p.Code != nil && len(*p.Code) > maxSubmittedCode {
			clipped := (*p.Code)[:maxSubmittedCode]
			p.Code = &clipped
		}
		if p.AIScore != nil {
			v := *p.AIScore
			if v < 0 {
				v = 0
			}
			if v > 100 {
				v = 100
			}
			p.AIScore = &v
		}
	}

	if err := s.svc.Participants.ReplaceAll(body.Participants); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(body.Participants)})
}
