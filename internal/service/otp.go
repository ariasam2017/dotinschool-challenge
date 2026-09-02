package service

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/config"
)

// OTP timing — 5-digit code, 2-minute lifetime, 60-second resend cooldown,
// 5 verification attempts. These are load-bearing product behaviour.
const (
	OTPTTL            = 2 * time.Minute
	OTPResendCooldown = 60 * time.Second
	OTPMaxAttempts    = 5
)

type otpRecord struct {
	code      string
	expiresAt time.Time
	attempts  int
	sentAt    time.Time
}

// OTPService holds one-time codes in memory (they live only minutes, so losing
// them on restart is harmless) and drives the SMS provider for delivery.
type OTPService struct {
	mu    sync.Mutex
	store map[string]*otpRecord

	sms SMSProvider
	cfg *config.Config
	log *logrus.Logger
}

func newOTPService(cfg *config.Config, sms SMSProvider, log *logrus.Logger) *OTPService {
	s := &OTPService{store: make(map[string]*otpRecord), sms: sms, cfg: cfg, log: log}
	go s.sweepLoop()
	return s
}

// generateCode returns a uniformly random 5-digit string ("10000".."99999").
func generateCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(90000))
	if err != nil {
		// crypto/rand failing is catastrophic; fall back to a fixed-width value.
		return "10000"
	}
	return strconv.FormatInt(n.Int64()+10000, 10)
}

// CooldownRemaining reports how long the caller must wait before a resend, or 0.
func (s *OTPService) CooldownRemaining(phone string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.store[phone]
	if rec == nil {
		return 0
	}
	elapsed := time.Since(rec.sentAt)
	if elapsed >= OTPResendCooldown {
		return 0
	}
	return OTPResendCooldown - elapsed
}

// Send delivers a fresh code for the given purpose ("register" or "login") and
// records it. The caller is responsible for the duplicate/not-found and
// cooldown checks that precede this.
func (s *OTPService) Send(phone, purpose string) error {
	code := generateCode()
	pattern := s.cfg.IppanelRegisterPattern
	if purpose == "login" {
		pattern = s.cfg.IppanelLoginPattern
	}
	if err := s.sms.SendPattern(phone, pattern, map[string]string{"code": code}); err != nil {
		s.log.Errorf("ارسال کد تأیید به %s شکست خورد: %v", phone, err)
		return apperr.New(502, "SMS_SEND_FAILED", "ارسال پیامک ناموفق بود، لطفاً دوباره تلاش کنید.")
	}
	now := time.Now()
	s.mu.Lock()
	s.store[phone] = &otpRecord{code: code, expiresAt: now.Add(OTPTTL), attempts: 0, sentAt: now}
	s.mu.Unlock()
	return nil
}

// Consume verifies and single-uses a code. On success the code is removed
// immediately. Mirrors the original consumeOtp semantics and error codes.
func (s *OTPService) Consume(phone, otp string) error {
	otp = trimSpace(otp)
	if otp == "" {
		return apperr.New(400, "OTP_REQUIRED", "لطفاً کد تأیید پیامک‌شده را وارد کنید.")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rec := s.store[phone]
	if rec == nil || time.Now().After(rec.expiresAt) {
		return apperr.New(400, "OTP_EXPIRED", "کد تأیید منقضی شده — دوباره درخواست کد کنید.")
	}
	rec.attempts++
	if rec.attempts > OTPMaxAttempts {
		delete(s.store, phone)
		return apperr.New(429, "OTP_TOO_MANY_ATTEMPTS", "تعداد تلاش‌های مجاز تمام شد — دوباره درخواست کد کنید.")
	}
	if !TimingSafeEqual(otp, rec.code) {
		return apperr.New(400, "INVALID_OTP", "کد تأیید نادرست است.")
	}
	delete(s.store, phone)
	return nil
}

// NotifyAdminOfNewRegistration fires the admin-only "someone just registered"
// SMS. Best effort: a failure is logged and never affects the registration.
func (s *OTPService) NotifyAdminOfNewRegistration(firstName, lastName string) {
	if s.cfg.IppanelAdminPhone == "" {
		return
	}
	go func() {
		err := s.sms.SendPattern(s.cfg.IppanelAdminPhone, s.cfg.IppanelAdminNotifyPattern,
			map[string]string{"name": firstName, "family": lastName})
		if err != nil {
			s.log.Errorf("اطلاع‌رسانی ثبت‌نام جدید به ادمین شکست خورد: %v", err)
		}
	}()
}

func (s *OTPService) sweepLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for k, rec := range s.store {
			if now.After(rec.expiresAt.Add(OTPResendCooldown)) {
				delete(s.store, k)
			}
		}
		s.mu.Unlock()
	}
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}
