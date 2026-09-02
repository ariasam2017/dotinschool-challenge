package web

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/apperr"
)

// POST /api/otp/request — public, rate-limited. Sends a 5-digit code by SMS.
func (s *server) handleOTPRequest(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("otp-request", c.ClientIP(), 5, 10*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}

	var body struct {
		Phone   string `json:"phone"`
		Purpose string `json:"purpose"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	if !validPhone(body.Phone) {
		fail(c, apperr.InvalidPhone())
		return
	}

	purpose := "register"
	if body.Purpose == "login" {
		purpose = "login"
	}

	exists := s.svc.Participants.ExistsByPhone(body.Phone)
	if purpose == "register" && exists {
		fail(c, apperr.New(http.StatusConflict, "DUPLICATE_PHONE", "شما قبلاً با این شماره موبایل در این رویداد ثبت‌نام کرده‌اید."))
		return
	}
	if purpose == "login" && !exists {
		fail(c, apperr.New(http.StatusNotFound, "NOT_FOUND", "شماره موبایلی با این مشخصات ثبت‌نام نشده — لطفاً ابتدا ثبت‌نام کنید."))
		return
	}

	if wait := s.svc.OTP.CooldownRemaining(body.Phone); wait > 0 {
		sec := int(math.Ceil(wait.Seconds()))
		fail(c, apperr.New(429, "OTP_COOLDOWN", "لطفاً "+strconv.Itoa(sec)+" ثانیه دیگر دوباره تلاش کنید."))
		return
	}

	if err := s.svc.OTP.Send(body.Phone, purpose); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/register — public, rate-limited. Creates the participant + session.
func (s *server) handleRegister(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("register", c.ClientIP(), 10, 15*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}

	var body struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Phone     string `json:"phone"`
		OTP       string `json:"otp"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	body.FirstName = strings.TrimSpace(body.FirstName)
	body.LastName = strings.TrimSpace(body.LastName)

	if err := validateRegistration(body.FirstName, body.LastName, body.Phone); err != nil {
		fail(c, err)
		return
	}
	if err := s.svc.OTP.Consume(body.Phone, body.OTP); err != nil {
		fail(c, err)
		return
	}
	if s.svc.Participants.ExistsByPhone(body.Phone) {
		fail(c, apperr.New(http.StatusConflict, "DUPLICATE_PHONE", "شما قبلاً با این شماره موبایل در این رویداد ثبت‌نام کرده‌اید."))
		return
	}

	p, err := s.svc.Participants.Create(body.FirstName, body.LastName, body.Phone)
	if err != nil {
		s.failWith(c, err)
		return
	}
	token := s.svc.Sessions.IssueParticipant(p.ID)
	s.svc.OTP.NotifyAdminOfNewRegistration(body.FirstName, body.LastName)

	c.JSON(http.StatusCreated, participantWithToken(p, token))
}

// POST /api/lookup — public, rate-limited. Returning-user login by phone + OTP.
func (s *server) handleLookup(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("lookup", c.ClientIP(), 20, 15*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}

	var body struct {
		Phone string `json:"phone"`
		OTP   string `json:"otp"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	if !validPhone(body.Phone) {
		fail(c, apperr.InvalidPhone())
		return
	}

	p, err := s.svc.Participants.GetByPhone(body.Phone)
	if err != nil {
		if s.notFound(err) {
			fail(c, apperr.New(http.StatusNotFound, "NOT_FOUND", "شرکت‌کننده‌ای با این شماره موبایل ثبت‌نام نکرده است."))
			return
		}
		s.failWith(c, err)
		return
	}
	// OTP is only consumed once the record is confirmed to exist — a wrong code
	// on an unknown phone still reports NOT_FOUND, matching the original order.
	if err := s.svc.OTP.Consume(body.Phone, body.OTP); err != nil {
		fail(c, err)
		return
	}

	token := s.svc.Sessions.IssueParticipant(p.ID)
	c.JSON(http.StatusOK, participantWithToken(p, token))
}

// POST /api/session — auto-login on a remembered device via a prior session
// token (already OTP-verified, so no new code needed).
func (s *server) handleSession(c *gin.Context) {
	var body struct {
		Token string `json:"token"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	id, aerr := s.resolveParticipant(body.Token)
	if aerr != nil {
		fail(c, aerr)
		return
	}
	p, err := s.svc.Participants.GetByID(id)
	if err != nil {
		if s.notFound(err) {
			fail(c, apperr.New(http.StatusNotFound, "NOT_FOUND", "حساب شرکت‌کننده یافت نشد."))
			return
		}
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, participantWithToken(p, body.Token))
}

// GET /api/config — public snapshot of event timing + published challenge.
func (s *server) handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.svc.Settings.CurrentConfig())
}

// ---- helpers ---------------------------------------------------------

func (s *server) notFound(err error) bool { return err == gorm.ErrRecordNotFound }

func validateRegistration(first, last, phone string) *apperr.Error {
	if first == "" || last == "" || phone == "" {
		return apperr.InvalidInput("لطفاً همه‌ی فیلدها را کامل وارد کنید.")
	}
	if len([]rune(first)) > 60 || len([]rune(last)) > 60 {
		return apperr.InvalidInput("طول یکی از فیلدها بیش از حد مجاز است.")
	}
	if !validPhone(phone) {
		return apperr.InvalidInput("شماره موبایل باید ۱۰ رقم و بعد از +۹۸ باشد.")
	}
	return nil
}
