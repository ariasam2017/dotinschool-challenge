package service

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/config"
	"github.com/dotinschool/dotinschool/internal/model"
)

// SettingService is the key/value config store: event timing, the published
// challenge, and the AI provider list. Values are persisted as JSON text in the
// settings table, exactly as the original `config` table did.
type SettingService struct {
	db  *gorm.DB
	cfg *config.Config
	log *logrus.Logger

	aiMu sync.Mutex // guards the sidecar read/write race
}

func newSettingService(cfg *config.Config, db *gorm.DB, log *logrus.Logger) *SettingService {
	return &SettingService{db: db, cfg: cfg, log: log}
}

// getRaw returns the stored JSON for a key, or (nil, false) when absent.
func (s *SettingService) getRaw(key string) (json.RawMessage, bool) {
	var row model.Setting
	if err := s.db.Where("key = ?", key).First(&row).Error; err != nil {
		return nil, false
	}
	return json.RawMessage(row.Value), true
}

// GetInto unmarshals a key's value into dst. Missing key => dst untouched, ok=false.
func (s *SettingService) GetInto(key string, dst any) (ok bool, err error) {
	raw, found := s.getRaw(key)
	if !found {
		return false, nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return false, err
	}
	return true, nil
}

// Set writes (insert or replace) a key with a JSON-encoded value.
func (s *SettingService) Set(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.db.Save(&model.Setting{Key: key, Value: string(raw)}).Error
}

// SetRaw writes an already-encoded JSON payload verbatim (used when a handler
// received e.g. a `challenge` object it validated but did not fully model).
func (s *SettingService) SetRaw(key string, raw json.RawMessage) error {
	return s.db.Save(&model.Setting{Key: key, Value: string(raw)}).Error
}

// ---- typed accessors -------------------------------------------------------

func (s *SettingService) nullableInt64(key string) *int64 {
	raw, ok := s.getRaw(key)
	if !ok {
		return nil
	}
	var v *int64
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}

// EventEndsAt is the epoch-ms end of the event, or nil.
func (s *SettingService) EventEndsAt() *int64 { return s.nullableInt64(model.KeyEventEndsAt) }

// EventStartsAt is the scheduled epoch-ms auto-start, or nil when not scheduled.
func (s *SettingService) EventStartsAt() *int64 { return s.nullableInt64(model.KeyEventStartsAt) }

// PausedRemainMs is the milliseconds that were left when the admin paused.
func (s *SettingService) PausedRemainMs() *int64 { return s.nullableInt64(model.KeyPausedRemainMs) }

// EventPaused reports whether the admin has paused the event.
func (s *SettingService) EventPaused() bool {
	raw, ok := s.getRaw(model.KeyEventPaused)
	if !ok {
		return false
	}
	var v bool
	_ = json.Unmarshal(raw, &v)
	return v
}

// Challenge returns the published problem statement, falling back to the
// default if the row is somehow missing or corrupt.
func (s *SettingService) Challenge() model.Challenge {
	raw, ok := s.getRaw(model.KeyChallenge)
	if !ok {
		return model.DefaultChallenge()
	}
	var c model.Challenge
	if err := json.Unmarshal(raw, &c); err != nil {
		return model.DefaultChallenge()
	}
	return c
}

// ChallengeRaw returns the stored challenge JSON as-is (nil when absent).
func (s *SettingService) ChallengeRaw() json.RawMessage {
	raw, _ := s.getRaw(model.KeyChallenge)
	return raw
}

// ConfigDTO is the exact payload GET /api/config returns.
type ConfigDTO struct {
	EventEndsAt    *int64          `json:"eventEndsAt"`
	EventStartsAt  *int64          `json:"eventStartsAt"`
	EventPaused    bool            `json:"eventPaused"`
	PausedRemainMs *int64          `json:"pausedRemainMs"`
	Challenge      json.RawMessage `json:"challenge"`
}

// CurrentConfig assembles the public event/challenge snapshot.
func (s *SettingService) CurrentConfig() ConfigDTO {
	ch := s.ChallengeRaw()
	if ch == nil {
		def, _ := json.Marshal(model.DefaultChallenge())
		ch = def
	}
	return ConfigDTO{
		EventEndsAt:    s.EventEndsAt(),
		EventStartsAt:  s.EventStartsAt(),
		EventPaused:    s.EventPaused(),
		PausedRemainMs: s.PausedRemainMs(),
		Challenge:      ch,
	}
}

// ---- AI settings with self-healing sidecar -------------------------------

// AISettings reads the panel provider list. If the DB row is gone but the
// data/ai-config.json sidecar still has it, it is restored to the DB on the
// spot so a connection is never lost mid-session.
func (s *SettingService) AISettings() model.AISettings {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()

	var out model.AISettings
	if ok, _ := s.GetInto(model.KeyAISettings, &out); ok && len(out.Providers) > 0 {
		return out
	}

	if parsed, err := s.readAISidecar(); err == nil && len(parsed.Providers) > 0 {
		_ = s.Set(model.KeyAISettings, parsed)
		return parsed
	}
	return model.AISettings{Providers: []model.AIProvider{}}
}

// SaveAISettings persists the provider list to the DB and to the sidecar file.
func (s *SettingService) SaveAISettings(v model.AISettings) error {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()
	if err := s.Set(model.KeyAISettings, v); err != nil {
		return err
	}
	s.writeAISidecar(v)
	return nil
}

// RestoreAISidecarIfMissing runs once at boot.
func (s *SettingService) RestoreAISidecarIfMissing() {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()

	var cur model.AISettings
	if ok, _ := s.GetInto(model.KeyAISettings, &cur); ok && len(cur.Providers) > 0 {
		return
	}
	parsed, err := s.readAISidecar()
	if err != nil || len(parsed.Providers) == 0 {
		return
	}
	if err := s.Set(model.KeyAISettings, parsed); err == nil {
		s.log.Infof("تنظیمات سرویس هوش مصنوعی از data/ai-config.json بازیابی شد (%d سرویس).", len(parsed.Providers))
	}
}

func (s *SettingService) readAISidecar() (model.AISettings, error) {
	var out model.AISettings
	b, err := os.ReadFile(s.cfg.AIConfigPath())
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	if len(out.Providers) == 0 {
		return out, errors.New("sidecar has no providers")
	}
	return out, nil
}

func (s *SettingService) writeAISidecar(v model.AISettings) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(s.cfg.AIConfigPath(), b, 0o600); err != nil {
		s.log.Errorf("نوشتن data/ai-config.json ناموفق بود: %v", err)
	}
}
