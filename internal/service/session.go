package service

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session TTLs — admin re-authenticates every 12h; a participant device stays
// remembered for 30 days on a sliding window.
const (
	AdminSessionTTL       = 12 * time.Hour
	ParticipantSessionTTL = 30 * 24 * time.Hour
)

type adminSession struct {
	username string
	at       time.Time
}

type participantSession struct {
	participantID string
	at            time.Time
}

// SessionService keeps admin and participant sessions in memory only. A server
// restart invalidates every session — the admin panel handles the resulting
// 401 by showing the login screen again.
type SessionService struct {
	mu           sync.Mutex
	admins       map[string]*adminSession
	participants map[string]*participantSession
}

func newSessionService() *SessionService {
	s := &SessionService{
		admins:       make(map[string]*adminSession),
		participants: make(map[string]*participantSession),
	}
	go s.sweepLoop()
	return s
}

// ---- admin --------------------------------------------------------------

// IssueAdmin creates a 12h admin session and returns its bearer token.
func (s *SessionService) IssueAdmin(username string) string {
	token := uuid.NewString()
	s.mu.Lock()
	s.admins[token] = &adminSession{username: username, at: time.Now()}
	s.mu.Unlock()
	return token
}

// ResolveAdmin validates a bearer token. An expired token is dropped.
func (s *SessionService) ResolveAdmin(token string) (username string, ok bool) {
	if token == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.admins[token]
	if rec == nil || time.Since(rec.at) > AdminSessionTTL {
		delete(s.admins, token)
		return "", false
	}
	return rec.username, true
}

// RevokeAdmin drops a token on logout.
func (s *SessionService) RevokeAdmin(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	delete(s.admins, token)
	s.mu.Unlock()
}

// ---- participant ------------------------------------------------------

// IssueParticipant creates a 30-day sliding session for a device.
func (s *SessionService) IssueParticipant(participantID string) string {
	token := uuid.NewString()
	s.mu.Lock()
	s.participants[token] = &participantSession{participantID: participantID, at: time.Now()}
	s.mu.Unlock()
	return token
}

// ResolveParticipant validates a session token and slides its expiry forward.
func (s *SessionService) ResolveParticipant(token string) (participantID string, ok bool) {
	if token == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.participants[token]
	if rec == nil || time.Since(rec.at) > ParticipantSessionTTL {
		delete(s.participants, token)
		return "", false
	}
	rec.at = time.Now() // sliding expiration
	return rec.participantID, true
}

func (s *SessionService) sweepLoop() {
	t := time.NewTicker(1 * time.Hour)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for tok, rec := range s.admins {
			if now.Sub(rec.at) > AdminSessionTTL {
				delete(s.admins, tok)
			}
		}
		for tok, rec := range s.participants {
			if now.Sub(rec.at) > ParticipantSessionTTL {
				delete(s.participants, tok)
			}
		}
		s.mu.Unlock()
	}
}
