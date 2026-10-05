package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
	"github.com/dotinschool/dotinschool/internal/service"
)

// ---- player (public, token in body) ----------------------------------------

type gameTokenBody struct {
	Token string `json:"token"`
}

// GET /api/game/info — public. What the lobby needs before registration.
func (s *server) handleGameInfo(c *gin.Context) {
	gs := s.svc.Game.Settings()
	c.Header("Cache-Control", "no-cache")
	c.JSON(http.StatusOK, gin.H{
		"open":               gs.Open,
		"questionsPerGame":   gs.QuestionsPerGame,
		"secondsPerQuestion": gs.SecondsPerQuestion,
	})
}

// POST /api/game/register — public, rate-limited. Name + mobile, no OTP: the
// game is a one-shot quiz, and a whole classroom often shares one NAT'd IP, so
// the limit here is generous.
func (s *server) handleGameRegister(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("game-register", c.ClientIP(), 200, 10*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}
	var body struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Phone     string `json:"phone"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	p, created, err := s.svc.Game.Register(body.FirstName, body.LastName, body.Phone)
	if err != nil {
		s.failWith(c, err)
		return
	}
	st, err := s.svc.Game.State(p.Token)
	if err != nil {
		s.failWith(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"token": p.Token, "created": created, "state": st})
}

func (s *server) gameAction(c *gin.Context, run func(token string) (*service.GameState, error)) {
	var body gameTokenBody
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	st, err := run(body.Token)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// POST /api/game/state — the player's current screen (refresh-safe).
func (s *server) handleGameState(c *gin.Context) { s.gameAction(c, s.svc.Game.State) }

// POST /api/game/start — deal questions and serve the first one.
func (s *server) handleGameStart(c *gin.Context) { s.gameAction(c, s.svc.Game.Start) }

// POST /api/game/next — serve the next question after a reveal.
func (s *server) handleGameNext(c *gin.Context) { s.gameAction(c, s.svc.Game.Next) }

// POST /api/game/replay — test mode only: clear my run and play again.
func (s *server) handleGameReplay(c *gin.Context) { s.gameAction(c, s.svc.Game.Replay) }

// POST /api/game/answer — {token, index, choice}; choice -1 = time ran out.
func (s *server) handleGameAnswer(c *gin.Context) {
	var body struct {
		Token  string `json:"token"`
		Index  int    `json:"index"`
		Choice int    `json:"choice"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	st, err := s.svc.Game.Answer(body.Token, body.Index, body.Choice)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// ---- admin -------------------------------------------------------------------

// GET /api/admin/game/board — live ranked leaderboard + podium.
func (s *server) handleGameBoard(c *gin.Context) {
	b, err := s.svc.Game.Board()
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, b)
}

// PUT /api/admin/game/settings
func (s *server) handlePutGameSettings(c *gin.Context) {
	var body model.GameSettings
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	gs, err := s.svc.Game.SaveSettings(body)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gs)
}

// GET /api/admin/game/players/:id — one player's answers.
func (s *server) handleGamePlayerReview(c *gin.Context) {
	r, err := s.svc.Game.PlayerReview(c.Param("id"))
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"review": r})
}

// POST /api/admin/game/players/:id/reset — let a player play again.
func (s *server) handleGameResetPlayer(c *gin.Context) {
	if err := s.svc.Game.ResetPlayer(c.Param("id")); err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/admin/game/players/:id
func (s *server) handleGameDeletePlayer(c *gin.Context) {
	ok, err := s.svc.Game.DeletePlayer(c.Param("id"))
	if err != nil {
		s.failWith(c, err)
		return
	}
	if !ok {
		fail(c, apperr.NotFound("بازیکن پیدا نشد."))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DELETE /api/admin/game/players — wipe every player/result.
func (s *server) handleGameDeleteAllPlayers(c *gin.Context) {
	n, err := s.svc.Game.DeleteAllPlayers()
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": n})
}

// GET /api/admin/game/questions
func (s *server) handleListGameQuestions(c *gin.Context) {
	list, err := s.svc.Game.ListQuestions()
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// POST /api/admin/game/questions
func (s *server) handleCreateGameQuestion(c *gin.Context) {
	var body service.QuestionInput
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	q, err := s.svc.Game.CreateQuestion(body)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusCreated, q)
}

// PUT /api/admin/game/questions/:id
func (s *server) handleUpdateGameQuestion(c *gin.Context) {
	var body service.QuestionInput
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	q, err := s.svc.Game.UpdateQuestion(c.Param("id"), body)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, q)
}

// DELETE /api/admin/game/questions/:id
func (s *server) handleDeleteGameQuestion(c *gin.Context) {
	n, err := s.svc.Game.DeleteQuestions([]string{c.Param("id")})
	if err != nil {
		s.failWith(c, err)
		return
	}
	if n == 0 {
		fail(c, apperr.NotFound("سوال پیدا نشد."))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/admin/game/questions/delete — bulk delete {ids}.
func (s *server) handleBulkDeleteGameQuestions(c *gin.Context) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	n, err := s.svc.Game.DeleteQuestions(body.IDs)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": n})
}

// POST /api/admin/game/questions/import — {questions, replace}.
func (s *server) handleImportGameQuestions(c *gin.Context) {
	var body struct {
		Questions []service.QuestionInput `json:"questions"`
		Replace   bool                    `json:"replace"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	added, skipped, err := s.svc.Game.ImportQuestions(body.Questions, body.Replace)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "added": added, "skipped": skipped})
}

// POST /api/admin/game/questions/defaults — add the built-in tech bank.
func (s *server) handleAddDefaultGameQuestions(c *gin.Context) {
	added, err := s.svc.Game.AddDefaultBank()
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "added": added})
}
