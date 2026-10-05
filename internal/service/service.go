// Package service holds the business logic layer: participants, settings,
// sessions, rate limiting, OTP/SMS, and the AI client. HTTP handlers depend on
// these; these depend only on the model, config, and the database handle.
package service

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/config"
)

// Services is the wired bundle handed to the web layer.
type Services struct {
	Config *config.Config
	DB     *gorm.DB
	Log    *logrus.Logger

	Settings     *SettingService
	Participants *ParticipantService
	Sessions     *SessionService
	RateLimiter  *RateLimiter
	OTP          *OTPService
	SMS          SMSProvider
	AI           *AIService
	ClientErrors *ClientErrorService
	SysStats     *SysStatsService
	Game         *GameService
}

// New constructs every service with its dependencies.
func New(cfg *config.Config, db *gorm.DB, log *logrus.Logger) *Services {
	settings := newSettingService(cfg, db, log)
	// Always the real provider — whether it's "configured" is resolved fresh on
	// every call (panel settings override .env, per field), so saving a key from
	// the admin panel takes effect immediately with no restart.
	var sms SMSProvider = newIppanelProvider(cfg, settings, log)

	return &Services{
		Config: cfg,
		DB:     db,
		Log:    log,

		Settings:     settings,
		Participants: newParticipantService(db),
		Sessions:     newSessionService(),
		RateLimiter:  newRateLimiter(),
		OTP:          newOTPService(cfg, sms, log),
		SMS:          sms,
		AI:           newAIService(cfg, settings, log),
		ClientErrors: newClientErrorService(db),
		SysStats:     newSysStatsService(),
		Game:         newGameService(db, settings),
	}
}

// RestoreAISidecarIfMissing is called once at boot.
func (s *Services) RestoreAISidecarIfMissing() {
	s.Settings.RestoreAISidecarIfMissing()
	s.Settings.RestoreSMSSidecarIfMissing()
}
