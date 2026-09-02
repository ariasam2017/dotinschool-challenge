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
}

// New constructs every service with its dependencies.
func New(cfg *config.Config, db *gorm.DB, log *logrus.Logger) *Services {
	settings := newSettingService(cfg, db, log)

	var sms SMSProvider
	if cfg.IppanelAPIKey != "" {
		sms = newIppanelProvider(cfg, log)
	} else {
		sms = nullSMSProvider{cfg: cfg}
	}

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
	}
}

// RestoreAISidecarIfMissing is called once at boot.
func (s *Services) RestoreAISidecarIfMissing() {
	s.Settings.RestoreAISidecarIfMissing()
}
