package service

import (
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool-challenge/internal/model"
)

// Scheduler flips a scheduled event live the moment eventStartsAt arrives. It
// checks frequently and, once the start time is in the past, clears
// eventStartsAt so the event is simply "live until eventEndsAt" — no manual
// action from the admin, and the countdown on both panels switches over on its
// own.
type Scheduler struct {
	settings *SettingService
	log      *logrus.Logger
	cron     *cron.Cron
}

// NewScheduler wires the cron runner (not yet started).
func NewScheduler(settings *SettingService, log *logrus.Logger) *Scheduler {
	return &Scheduler{
		settings: settings,
		log:      log,
		cron:     cron.New(),
	}
}

// Start begins the every-15-seconds check.
func (s *Scheduler) Start() {
	_, err := s.cron.AddFunc("@every 15s", s.tick)
	if err != nil {
		s.log.WithError(err).Warn("could not register the auto-start cron job")
		return
	}
	s.cron.Start()
	// Run once immediately so a start time that already passed while the process
	// was down is honoured without waiting for the first tick.
	go s.tick()
}

// Stop halts the cron runner and waits for a running job to finish.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

func (s *Scheduler) tick() {
	startsAt := s.settings.EventStartsAt()
	if startsAt == nil {
		return
	}
	if *startsAt > time.Now().UnixMilli() {
		return // still in the future — keep waiting
	}
	// The scheduled moment has arrived: drop the schedule marker and make sure
	// the event is not left paused.
	if err := s.settings.Set(model.KeyEventStartsAt, nil); err != nil {
		s.log.WithError(err).Warn("auto-start: could not clear eventStartsAt")
		return
	}
	if s.settings.EventPaused() {
		_ = s.settings.Set(model.KeyEventPaused, false)
	}
	s.log.Info("رویداد طبق زمان‌بندی به‌صورت خودکار زنده شد")
}
