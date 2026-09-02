package web

import (
	"encoding/json"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
)

// phoneRe matches a bare Iranian mobile number: "9XXXXXXXXX" (no leading zero,
// no +98).
var phoneRe = regexp.MustCompile(`^9\d{9}$`)

func validPhone(p string) bool { return phoneRe.MatchString(p) }

// participantWithToken marshals a participant to the exact wire shape the
// original returned, then folds in the session token — matching
// Object.assign({ sessionToken }, rowToParticipant(row)).
func participantWithToken(p *model.Participant, token string) gin.H {
	raw, _ := json.Marshal(p)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["sessionToken"] = token
	return m
}

// resolveParticipant validates a participant session token or returns the
// SESSION_EXPIRED error the original used.
func (s *server) resolveParticipant(token string) (string, *apperr.Error) {
	id, ok := s.svc.Sessions.ResolveParticipant(token)
	if !ok {
		return "", apperr.New(401, "SESSION_EXPIRED", "نشست شما منقضی شده — لطفاً دوباره وارد شوید.")
	}
	return id, nil
}
