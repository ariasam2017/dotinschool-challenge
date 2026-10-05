package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/fnv"
	mrand "math/rand/v2"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
)

// answerGraceMs absorbs network latency: an answer that reaches the server up
// to this long after the question's deadline still counts (timed at the limit).
const answerGraceMs = 1500

// GameService runs the Quiz-of-Kings style individual quiz: the question
// bank, player registration, the server-timed game loop, and the ranking.
//
// Every scoring decision is made here — the correct option is only revealed
// after the player has answered, and the per-question time is measured from
// the moment the server served the question, so a client cannot cheat either.
type GameService struct {
	db       *gorm.DB
	settings *SettingService
	now      func() time.Time

	// Concurrency: every state transition of one run (start/answer/next, the
	// lazy timeout inside State, an admin reset) holds that player's stripe
	// lock, so a double-tap or a second tab can never answer the same question
	// twice — while different players proceed in parallel. Stripes (not one
	// mutex per player) keep memory bounded however many register.
	stripes [64]sync.Mutex
	regMu   sync.Mutex // registration: phone uniqueness check + insert

	// settings are read on every request; cache them (SaveSettings refreshes)
	cfg atomic.Pointer[model.GameSettings]

	// The question decks: per difficulty, a shuffled list of bank ids dealt out
	// in order, so consecutive players get different questions before any
	// repeats.
	deckMu  sync.Mutex
	decks   map[string][]string // difficulty -> ids
	deckRev int64
	bankRev atomic.Int64 // bumped on every bank change; a stale deck is rebuilt
}

func newGameService(db *gorm.DB, settings *SettingService) *GameService {
	return &GameService{db: db, settings: settings, now: time.Now}
}

func (s *GameService) stripe(playerID string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(playerID))
	return &s.stripes[h.Sum32()%uint32(len(s.stripes))]
}

// lockPlayer resolves a token, takes that player's stripe lock and re-reads
// the row under it (so the caller sees the latest state). Call unlock when done.
func (s *GameService) lockPlayer(token string) (p *model.GamePlayer, unlock func(), err error) {
	first, err := s.byToken(token)
	if err != nil {
		return nil, nil, err
	}
	mu := s.stripe(first.ID)
	mu.Lock()
	var cur model.GamePlayer
	if err := s.db.Where("id = ?", first.ID).First(&cur).Error; err != nil {
		mu.Unlock()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, gameSessionErr() // deleted meanwhile
		}
		return nil, nil, err
	}
	return &cur, mu.Unlock, nil
}

func (s *GameService) nowMs() int64 { return s.now().UnixMilli() }

// ---- settings ---------------------------------------------------------------

// Settings returns the game settings (cached in memory) with defaults filled in.
func (s *GameService) Settings() model.GameSettings {
	if c := s.cfg.Load(); c != nil {
		return *c
	}
	gs := s.loadSettings()
	s.cfg.Store(&gs)
	return gs
}

func (s *GameService) loadSettings() model.GameSettings {
	gs := model.DefaultGameSettings()
	_, _ = s.settings.GetInto(model.KeyGameSettings, &gs)
	if gs.QuestionsPerGame < 1 {
		gs.QuestionsPerGame = model.DefaultGameSettings().QuestionsPerGame
	}
	if gs.SecondsPerQuestion < 5 {
		gs.SecondsPerQuestion = model.DefaultGameSettings().SecondsPerQuestion
	}
	// settings saved before the reveal animation existed have zeros here
	def := model.DefaultGameSettings()
	for _, f := range []struct {
		v *int
		d int
	}{
		{&gs.RevealThirdSec, def.RevealThirdSec},
		{&gs.RevealSecondSec, def.RevealSecondSec},
		{&gs.RevealFirstSec, def.RevealFirstSec},
		{&gs.RevealGapSec, def.RevealGapSec},
	} {
		if *f.v <= 0 {
			*f.v = f.d
		}
	}
	return gs
}

// SaveSettings validates and stores the game settings.
func (s *GameService) SaveSettings(gs model.GameSettings) (model.GameSettings, error) {
	if gs.QuestionsPerGame < 1 || gs.QuestionsPerGame > 50 {
		return gs, apperr.InvalidInput("تعداد سوال هر بازی باید بین ۱ تا ۵۰ باشد.")
	}
	if gs.SecondsPerQuestion < 5 || gs.SecondsPerQuestion > 300 {
		return gs, apperr.InvalidInput("زمان هر سوال باید بین ۵ تا ۳۰۰ ثانیه باشد.")
	}
	for _, v := range []int{gs.RevealThirdSec, gs.RevealSecondSec, gs.RevealFirstSec} {
		if v < 2 || v > 120 {
			return gs, apperr.InvalidInput("زمان انیمیشن معرفی هر نفر باید بین ۲ تا ۱۲۰ ثانیه باشد.")
		}
	}
	if gs.RevealGapSec < 0 || gs.RevealGapSec > 60 {
		return gs, apperr.InvalidInput("مکث بین معرفی نفرات باید بین ۰ تا ۶۰ ثانیه باشد.")
	}
	gs.FinishMessage = strings.TrimSpace(gs.FinishMessage)
	if len([]rune(gs.FinishMessage)) > 500 {
		return gs, apperr.InvalidInput("پیام پایان بازی حداکثر ۵۰۰ کاراکتر است.")
	}
	if err := s.settings.Set(model.KeyGameSettings, gs); err != nil {
		return gs, err
	}
	fresh := s.loadSettings()
	s.cfg.Store(&fresh)
	return fresh, nil
}

// ---- question bank ----------------------------------------------------------

// QuestionDTO is the admin view of a bank question.
type QuestionDTO struct {
	ID         string    `json:"id"`
	Category   string    `json:"category"`
	Difficulty string    `json:"difficulty"`
	Text       string    `json:"text"`
	Options    [4]string `json:"options"`
	Correct    int       `json:"correct"`
	CreatedAt  int64     `json:"createdAt"`
}

// QuestionInput is what the admin editor/import sends.
type QuestionInput struct {
	Category   string    `json:"category"`
	Difficulty string    `json:"difficulty"` // easy | hard (empty = easy)
	Text       string    `json:"text"`
	Options    [4]string `json:"options"`
	Correct    int       `json:"correct"`
}

func toQuestionDTO(q model.GameQuestion) QuestionDTO {
	return QuestionDTO{ID: q.ID, Category: q.Category, Difficulty: q.Difficulty, Text: q.Text, Options: q.Options(), Correct: q.Correct, CreatedAt: q.CreatedAt}
}

// normalizeQuestion trims and validates one question.
func normalizeQuestion(in QuestionInput) (QuestionInput, *apperr.Error) {
	in.Text = strings.TrimSpace(in.Text)
	in.Category = strings.TrimSpace(in.Category)
	if in.Category == "" {
		in.Category = model.DefaultGameCategory
	}
	switch strings.ToLower(strings.TrimSpace(in.Difficulty)) {
	case "", "easy", "آسان":
		in.Difficulty = model.DifficultyEasy
	case "hard", "سخت":
		in.Difficulty = model.DifficultyHard
	default:
		return in, apperr.InvalidInput("سطح سوال باید «آسان» یا «سخت» باشد.")
	}
	if in.Text == "" {
		return in, apperr.InvalidInput("متن سوال نمی‌تواند خالی باشد.")
	}
	if len([]rune(in.Text)) > 600 {
		return in, apperr.InvalidInput("متن سوال حداکثر ۶۰۰ کاراکتر است.")
	}
	if len([]rune(in.Category)) > 40 {
		return in, apperr.InvalidInput("نام دسته حداکثر ۴۰ کاراکتر است.")
	}
	seen := map[string]bool{}
	for i := range in.Options {
		in.Options[i] = strings.TrimSpace(in.Options[i])
		if in.Options[i] == "" {
			return in, apperr.InvalidInput("هر چهار گزینه باید پر شوند.")
		}
		if len([]rune(in.Options[i])) > 200 {
			return in, apperr.InvalidInput("متن هر گزینه حداکثر ۲۰۰ کاراکتر است.")
		}
		if seen[in.Options[i]] {
			return in, apperr.InvalidInput("گزینه‌های تکراری مجاز نیستند.")
		}
		seen[in.Options[i]] = true
	}
	if in.Correct < 0 || in.Correct > 3 {
		return in, apperr.InvalidInput("گزینه‌ی درست را مشخص کنید.")
	}
	return in, nil
}

// ListQuestions returns the whole bank, newest first.
func (s *GameService) ListQuestions() ([]QuestionDTO, error) {
	var rows []model.GameQuestion
	if err := s.db.Order("created_at DESC, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]QuestionDTO, len(rows))
	for i, q := range rows {
		out[i] = toQuestionDTO(q)
	}
	return out, nil
}

// QuestionCount is the size of the bank.
func (s *GameService) QuestionCount() int64 {
	var n int64
	s.db.Model(&model.GameQuestion{}).Count(&n)
	return n
}

// CreateQuestion validates and inserts one question.
func (s *GameService) CreateQuestion(in QuestionInput) (QuestionDTO, error) {
	in, aerr := normalizeQuestion(in)
	if aerr != nil {
		return QuestionDTO{}, aerr
	}
	q := model.GameQuestion{ID: uuid.NewString(), Category: in.Category, Difficulty: in.Difficulty, Text: in.Text, Correct: in.Correct, CreatedAt: s.nowMs()}
	q.SetOptions(in.Options)
	if err := s.db.Create(&q).Error; err != nil {
		return QuestionDTO{}, err
	}
	s.bankRev.Add(1)
	return toQuestionDTO(q), nil
}

// UpdateQuestion replaces one question's content. Games already dealt keep
// their snapshot, so editing never changes a played result.
func (s *GameService) UpdateQuestion(id string, in QuestionInput) (QuestionDTO, error) {
	in, aerr := normalizeQuestion(in)
	if aerr != nil {
		return QuestionDTO{}, aerr
	}
	var q model.GameQuestion
	if err := s.db.Where("id = ?", id).First(&q).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return QuestionDTO{}, apperr.NotFound("سوال پیدا نشد.")
		}
		return QuestionDTO{}, err
	}
	q.Category, q.Difficulty, q.Text, q.Correct = in.Category, in.Difficulty, in.Text, in.Correct
	q.SetOptions(in.Options)
	if err := s.db.Save(&q).Error; err != nil {
		return QuestionDTO{}, err
	}
	s.bankRev.Add(1) // the difficulty may have moved it to the other deck
	return toQuestionDTO(q), nil
}

// DeleteQuestions removes the given questions; returns how many were deleted.
func (s *GameService) DeleteQuestions(ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := s.db.Where("id IN ?", ids).Delete(&model.GameQuestion{})
	s.bankRev.Add(1)
	return res.RowsAffected, res.Error
}

// ImportQuestions validates every row up front, then inserts them all in one
// transaction (optionally replacing the whole bank). Rows whose text already
// exists in the bank are skipped unless replacing.
func (s *GameService) ImportQuestions(in []QuestionInput, replace bool) (added int, skipped int, err error) {
	if len(in) == 0 {
		return 0, 0, apperr.InvalidInput("فایل هیچ سوالی ندارد.")
	}
	if len(in) > 5000 {
		return 0, 0, apperr.InvalidInput("حداکثر ۵۰۰۰ سوال در هر بار ورود مجاز است.")
	}
	clean := make([]QuestionInput, 0, len(in))
	for i, q := range in {
		n, aerr := normalizeQuestion(q)
		if aerr != nil {
			return 0, 0, apperr.InvalidInput("سوال شماره‌ی " + strconv.Itoa(i+1) + ": " + aerr.Message)
		}
		clean = append(clean, n)
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		existing := map[string]bool{}
		if replace {
			if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.GameQuestion{}).Error; err != nil {
				return err
			}
		} else {
			var texts []string
			if err := tx.Model(&model.GameQuestion{}).Pluck("text", &texts).Error; err != nil {
				return err
			}
			for _, t := range texts {
				existing[t] = true
			}
		}
		base := s.nowMs()
		rows := make([]model.GameQuestion, 0, len(clean))
		for i, q := range clean {
			if existing[q.Text] {
				skipped++
				continue
			}
			existing[q.Text] = true
			r := model.GameQuestion{ID: uuid.NewString(), Category: q.Category, Difficulty: q.Difficulty, Text: q.Text, Correct: q.Correct, CreatedAt: base - int64(i)}
			r.SetOptions(q.Options)
			rows = append(rows, r)
		}
		added = len(rows)
		if len(rows) == 0 {
			return nil
		}
		return tx.CreateInBatches(rows, 200).Error
	})
	s.bankRev.Add(1)
	return added, skipped, err
}

// AddDefaultBank adds every built-in technology question not already present.
func (s *GameService) AddDefaultBank() (added int, err error) {
	defs := model.DefaultGameQuestions()
	in := make([]QuestionInput, len(defs))
	for i, q := range defs {
		in[i] = QuestionInput{Category: q.Category, Difficulty: q.Difficulty, Text: q.Text, Options: q.Options(), Correct: q.Correct}
	}
	added, _, err = s.ImportQuestions(in, false)
	return added, err
}

// ---- players ----------------------------------------------------------------

var (
	gamePhoneRe = regexp.MustCompile(`^9\d{9}$`)
	faDigits    = strings.NewReplacer(
		"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4", "۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
		"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9",
	)
)

// NormalizePhone turns "09121234567", "+98 912 123 4567", "۰۹۱۲…" etc. into
// the bare "9121234567" form. Returns "" when it is not an Iranian mobile.
func NormalizePhone(raw string) string {
	p := faDigits.Replace(raw)
	var b strings.Builder
	for _, r := range p {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	p = b.String()
	switch {
	case strings.HasPrefix(p, "0098"):
		p = p[4:]
	case strings.HasPrefix(p, "98") && len(p) == 12:
		p = p[2:]
	case strings.HasPrefix(p, "0"):
		p = p[1:]
	}
	if !gamePhoneRe.MatchString(p) {
		return ""
	}
	return p
}

func sameName(a, b string) bool {
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "‌", "")
		s = strings.Join(strings.Fields(s), "")
		s = strings.NewReplacer("ي", "ی", "ك", "ک").Replace(s)
		return strings.ToLower(s)
	}
	return norm(a) == norm(b)
}

func newToken() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Register creates a player, or — when the same person registers again with the
// same phone and name (a new device, cleared browser) — signs them back into
// their existing record. created reports which happened.
func (s *GameService) Register(first, last, phone string) (p *model.GamePlayer, created bool, err error) {
	first, last = strings.TrimSpace(first), strings.TrimSpace(last)
	if first == "" || last == "" {
		return nil, false, apperr.InvalidInput("لطفاً نام و نام خانوادگی را کامل وارد کنید.")
	}
	if len([]rune(first)) > 60 || len([]rune(last)) > 60 {
		return nil, false, apperr.InvalidInput("طول نام یا نام خانوادگی بیش از حد مجاز است.")
	}
	norm := NormalizePhone(phone)
	if norm == "" {
		return nil, false, apperr.New(400, "INVALID_PHONE", "شماره موبایل معتبر نیست — مثلاً ۰۹۱۲۱۲۳۴۵۶۷")
	}

	s.regMu.Lock()
	defer s.regMu.Unlock()

	var existing model.GamePlayer
	err = s.db.Where("phone = ?", norm).First(&existing).Error
	if err == nil {
		if sameName(existing.FirstName, first) && sameName(existing.LastName, last) {
			return &existing, false, nil
		}
		return nil, false, apperr.New(http.StatusConflict, "PHONE_TAKEN",
			"این شماره موبایل قبلاً با نام دیگری در بازی ثبت شده است.")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	p = &model.GamePlayer{
		ID:        uuid.NewString(),
		Token:     newToken(),
		FirstName: first,
		LastName:  last,
		Phone:     norm,
		CreatedAt: s.nowMs(),
		Status:    model.GameRegistered,
		Items:     "[]",
	}
	if err := s.db.Create(p).Error; err != nil {
		return nil, false, err
	}
	return p, true, nil
}

func (s *GameService) byToken(token string) (*model.GamePlayer, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, gameSessionErr()
	}
	var p model.GamePlayer
	if err := s.db.Where("token = ?", token).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gameSessionErr()
		}
		return nil, err
	}
	return &p, nil
}

func gameSessionErr() *apperr.Error {
	return apperr.New(401, "GAME_SESSION_EXPIRED", "نشست بازی شما معتبر نیست — لطفاً دوباره ثبت‌نام کنید.")
}

func loadItems(p *model.GamePlayer) []model.GameItem {
	var items []model.GameItem
	_ = json.Unmarshal([]byte(p.Items), &items)
	return items
}

func storeItems(p *model.GamePlayer, items []model.GameItem) {
	raw, _ := json.Marshal(items)
	p.Items = string(raw)
}

// ---- the game loop ----------------------------------------------------------

// GameQuestionView is the question as a player sees it (no answer key).
type GameQuestionView struct {
	Index       int       `json:"index"`
	Category    string    `json:"category"`
	Difficulty  string    `json:"difficulty"`
	Text        string    `json:"text"`
	Options     [4]string `json:"options"`
	LimitMs     int64     `json:"limitMs"`
	RemainingMs int64     `json:"remainingMs"`
}

// GameReveal is the outcome of an answered question.
type GameReveal struct {
	Index     int  `json:"index"`
	Choice    int  `json:"choice"`  // -1 = time ran out
	Correct   int  `json:"correct"` // display index of the right option
	IsCorrect bool `json:"isCorrect"`
	TimedOut  bool `json:"timedOut"`
}

// GameResult is the player's final scorecard. Answer time and score are
// deliberately left out: players only learn how many they got right; the
// timing and the score are for the admin board.
type GameResult struct {
	Correct       int    `json:"correct"`
	Total         int    `json:"total"`
	FinishedAt    int64  `json:"finishedAt"`
	FinishMessage string `json:"finishMessage"`
}

// GameReviewItem is one played question shown on the result screen.
type GameReviewItem struct {
	Category   string    `json:"category"`
	Difficulty string    `json:"difficulty,omitempty"`
	Text       string    `json:"text"`
	Options    [4]string `json:"options"`
	Choice     int       `json:"choice"`
	Correct    int       `json:"correct"`
	IsCorrect  bool      `json:"isCorrect"`
	Ms         int64     `json:"ms,omitempty"` // admin review only
}

// GamePlayerView is the public part of a player record.
type GamePlayerView struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone"`
	Status    string `json:"status"`
}

// GameState is everything the player screen needs, in one payload.
type GameState struct {
	Player             GamePlayerView    `json:"player"`
	Phase              string            `json:"phase"` // ready | question | reveal | finished
	Open               bool              `json:"open"`
	TestMode           bool              `json:"testMode"`
	QuestionsPerGame   int               `json:"questionsPerGame"`
	SecondsPerQuestion int               `json:"secondsPerQuestion"`
	Total              int               `json:"total"`
	Index              int               `json:"index"`
	Progress           []string          `json:"progress"` // per question: correct | wrong | pending
	Question           *GameQuestionView `json:"question"`
	Reveal             *GameReveal       `json:"reveal"`
	Result             *GameResult       `json:"result"`
	Review             []GameReviewItem  `json:"review"`
}

// State returns the player's current screen, first recording a timeout if the
// served question's deadline passed while they were away.
func (s *GameService) State(token string) (*GameState, error) {
	p, unlock, err := s.lockPlayer(token)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if p.Status == model.GamePlaying {
		items := loadItems(p)
		if p.Current < len(items) {
			it := items[p.Current]
			if it.ServedAt > 0 && !it.Answered && s.nowMs()-it.ServedAt > p.LimitMs+answerGraceMs {
				s.record(p, items, -1)
				if err := s.db.Save(p).Error; err != nil {
					return nil, err
				}
			}
		}
	}
	return s.view(p), nil
}

// Start deals a fresh random set of questions and serves the first one.
func (s *GameService) Start(token string) (*GameState, error) {
	p, unlock, err := s.lockPlayer(token)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if p.Status != model.GameRegistered {
		return s.view(p), nil // already started/finished: idempotent
	}
	gs := s.Settings()
	if !gs.Open {
		return nil, apperr.New(423, "GAME_CLOSED", "بازی هنوز توسط مدیر باز نشده — کمی بعد دوباره تلاش کن.")
	}

	bank, err := s.deal(gs.QuestionsPerGame)
	if err != nil {
		return nil, err
	}
	if len(bank) == 0 {
		return nil, apperr.New(423, "GAME_NO_QUESTIONS", "هنوز سوالی برای بازی ثبت نشده است.")
	}
	n := len(bank)

	items := make([]model.GameItem, n)
	for i := 0; i < n; i++ {
		q := bank[i]
		orig := q.Options()
		perm := mrand.Perm(4) // perm[display] = original index
		var opts [4]string
		correct := 0
		for d, o := range perm {
			opts[d] = orig[o]
			if o == q.Correct {
				correct = d
			}
		}
		items[i] = model.GameItem{QID: q.ID, Category: q.Category, Difficulty: q.Difficulty, Text: q.Text, Options: opts, Correct: correct, Choice: -1}
	}
	now := s.nowMs()
	items[0].ServedAt = now

	p.Status = model.GamePlaying
	p.StartedAt = now
	p.LimitMs = int64(gs.SecondsPerQuestion) * 1000
	p.Current = 0
	p.Total = n
	p.Correct, p.Answered, p.TotalMs, p.Score = 0, 0, 0, 0
	storeItems(p, items)
	if err := s.db.Save(p).Error; err != nil {
		return nil, err
	}
	return s.view(p), nil
}

// Answer records the player's choice (0..3, or -1 when their timer ran out) for
// question index. Re-answering an already answered question is a no-op that
// returns the current state, so a double tap is harmless.
func (s *GameService) Answer(token string, index, choice int) (*GameState, error) {
	p, unlock, err := s.lockPlayer(token)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if p.Status != model.GamePlaying || index != p.Current {
		return s.view(p), nil
	}
	items := loadItems(p)
	if p.Current >= len(items) || items[p.Current].Answered || items[p.Current].ServedAt == 0 {
		return s.view(p), nil
	}
	if choice < -1 || choice > 3 {
		choice = -1
	}
	s.record(p, items, choice)
	if err := s.db.Save(p).Error; err != nil {
		return nil, err
	}
	return s.view(p), nil
}

// Next serves the following question once the current one has been answered.
func (s *GameService) Next(token string) (*GameState, error) {
	p, unlock, err := s.lockPlayer(token)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if p.Status != model.GamePlaying {
		return s.view(p), nil
	}
	items := loadItems(p)
	if p.Current < len(items)-1 && items[p.Current].Answered {
		p.Current++
		items[p.Current].ServedAt = s.nowMs()
		storeItems(p, items)
		if err := s.db.Save(p).Error; err != nil {
			return nil, err
		}
	}
	return s.view(p), nil
}

// record scores the current question (choice -1 = timeout) and finishes the
// game after the last one. Caller holds the player's lock and saves p.
func (s *GameService) record(p *model.GamePlayer, items []model.GameItem, choice int) {
	it := &items[p.Current]
	elapsed := s.nowMs() - it.ServedAt
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > p.LimitMs+answerGraceMs {
		choice = -1 // arrived too late — counts as a timeout
	}
	ms := elapsed
	if choice == -1 || ms > p.LimitMs {
		ms = p.LimitMs
	}
	it.Answered = true
	it.Choice = choice
	it.IsCorrect = choice >= 0 && choice == it.Correct
	it.Ms = ms

	p.Answered++
	p.TotalMs += ms
	if it.IsCorrect {
		p.Correct++
	}
	if p.Current >= len(items)-1 {
		p.Status = model.GameFinished
		p.FinishedAt = s.nowMs()
		p.Score = ComputeGameScore(p.Correct, p.Total, p.TotalMs, p.LimitMs)
	}
	storeItems(p, items)
}

// deal draws n distinct questions in the easy/hard order of
// model.DifficultyPlan(n) (e.g. 5 = E E E H H). Each difficulty has its own
// deck: every bank id of that level in random order, consumed front to back
// and refilled with a fresh shuffle when it runs low (or after the bank
// changed), so consecutive players get different questions before any
// repeat. If the bank lacks enough questions of one level, the other level
// fills the gap — a game is never short of questions.
func (s *GameService) deal(n int) ([]model.GameQuestion, error) {
	s.deckMu.Lock()
	defer s.deckMu.Unlock()

	if rev := s.bankRev.Load(); rev != s.deckRev || s.decks == nil {
		s.decks, s.deckRev = map[string][]string{}, rev
	}
	refill := func(level string) error {
		var ids []string
		if err := s.db.Model(&model.GameQuestion{}).Where("difficulty = ?", level).Pluck("id", &ids).Error; err != nil {
			return err
		}
		mrand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		s.decks[level] = append(s.decks[level], ids...)
		return nil
	}
	seen := map[string]bool{}
	// take draws up to k distinct, not-yet-picked ids of one level
	take := func(level string, k int) ([]string, error) {
		if len(s.decks[level]) < k {
			if err := refill(level); err != nil {
				return nil, err
			}
		}
		got := make([]string, 0, k)
		rest := s.decks[level][:0:0]
		for _, id := range s.decks[level] {
			if len(got) < k && !seen[id] {
				got = append(got, id)
				seen[id] = true
			} else {
				rest = append(rest, id)
			}
		}
		s.decks[level] = rest
		return got, nil
	}

	plan := model.DifficultyPlan(n)
	need := map[string]int{}
	for _, lv := range plan {
		need[lv]++
	}
	pools := map[string][]string{}
	for _, lv := range []string{model.DifficultyEasy, model.DifficultyHard} {
		ids, err := take(lv, need[lv])
		if err != nil {
			return nil, err
		}
		pools[lv] = ids
	}
	// a level short of questions borrows from the other one
	for _, lv := range []string{model.DifficultyEasy, model.DifficultyHard} {
		other := model.DifficultyHard
		if lv == model.DifficultyHard {
			other = model.DifficultyEasy
		}
		if short := need[lv] - len(pools[lv]); short > 0 {
			extra, err := take(other, short)
			if err != nil {
				return nil, err
			}
			pools[lv] = append(pools[lv], extra...)
		}
	}
	// lay the ids out in plan order
	picked := make([]string, 0, n)
	for _, lv := range plan {
		if len(pools[lv]) > 0 {
			picked = append(picked, pools[lv][0])
			pools[lv] = pools[lv][1:]
		}
	}
	if len(picked) == 0 {
		return nil, nil
	}

	var rows []model.GameQuestion
	if err := s.db.Where("id IN ?", picked).Find(&rows).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]model.GameQuestion, len(rows))
	for _, q := range rows {
		byID[q.ID] = q
	}
	out := make([]model.GameQuestion, 0, len(picked))
	for _, id := range picked {
		if q, ok := byID[id]; ok {
			out = append(out, q)
		}
	}
	return out, nil
}

// ComputeGameScore: 100 points per correct answer plus a speed bonus of up to
// 99 for the time left over across the whole game. The bonus can never make up
// for a missing correct answer, so more correct answers always rank higher and,
// among equals, the faster player wins. A game with no correct answer scores 0.
func ComputeGameScore(correct, total int, totalMs, limitMs int64) int {
	if correct <= 0 {
		return 0
	}
	maxMs := int64(total) * limitMs
	bonus := int64(0)
	if maxMs > 0 {
		bonus = 99 * (maxMs - totalMs) / maxMs
	}
	if bonus < 0 {
		bonus = 0
	}
	if bonus > 99 {
		bonus = 99
	}
	return correct*100 + int(bonus)
}

func (s *GameService) view(p *model.GamePlayer) *GameState {
	gs := s.Settings()
	st := &GameState{
		Player:             GamePlayerView{ID: p.ID, FirstName: p.FirstName, LastName: p.LastName, Phone: p.Phone, Status: p.Status},
		Open:               gs.Open,
		TestMode:           gs.TestMode,
		QuestionsPerGame:   gs.QuestionsPerGame,
		SecondsPerQuestion: gs.SecondsPerQuestion,
		Total:              p.Total,
		Index:              p.Current,
		Progress:           []string{},
	}
	items := loadItems(p)
	for _, it := range items {
		switch {
		case !it.Answered:
			st.Progress = append(st.Progress, "pending")
		case it.IsCorrect:
			st.Progress = append(st.Progress, "correct")
		default:
			st.Progress = append(st.Progress, "wrong")
		}
	}

	if p.Status == model.GameRegistered || len(items) == 0 {
		st.Phase = "ready"
		st.Total = gs.QuestionsPerGame
		return st
	}

	cur := p.Current
	if cur >= len(items) {
		cur = len(items) - 1
	}
	it := items[cur]
	qv := &GameQuestionView{Index: cur, Category: it.Category, Difficulty: it.Difficulty, Text: it.Text, Options: it.Options, LimitMs: p.LimitMs}
	st.Question = qv

	if it.Answered {
		st.Reveal = &GameReveal{Index: cur, Choice: it.Choice, Correct: it.Correct, IsCorrect: it.IsCorrect, TimedOut: it.Choice == -1}
		st.Phase = "reveal"
	} else {
		rem := p.LimitMs - (s.nowMs() - it.ServedAt)
		if rem < 0 {
			rem = 0
		}
		qv.RemainingMs = rem
		st.Phase = "question"
	}

	if p.Status == model.GameFinished {
		st.Phase = "finished"
		st.Result = &GameResult{Correct: p.Correct, Total: p.Total, FinishedAt: p.FinishedAt, FinishMessage: gs.FinishMessage}
		st.Review = reviewOf(items)
		for i := range st.Review {
			st.Review[i].Ms = 0 // the player never sees per-question timing
		}
	}
	return st
}

func reviewOf(items []model.GameItem) []GameReviewItem {
	out := make([]GameReviewItem, 0, len(items))
	for _, it := range items {
		out = append(out, GameReviewItem{Category: it.Category, Difficulty: it.Difficulty, Text: it.Text, Options: it.Options, Choice: it.Choice, Correct: it.Correct, IsCorrect: it.IsCorrect, Ms: it.Ms})
	}
	return out
}

// ---- admin board ------------------------------------------------------------

// BoardRow is one player on the live leaderboard.
type BoardRow struct {
	Rank       int    `json:"rank"` // 1-based among finished players, 0 otherwise
	ID         string `json:"id"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Phone      string `json:"phone"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
	Correct    int    `json:"correct"`
	Answered   int    `json:"answered"`
	Total      int    `json:"total"`
	TotalMs    int64  `json:"totalMs"`
	Score      int    `json:"score"`
}

// BoardStats are the headline counters.
type BoardStats struct {
	Registered int     `json:"registered"`
	Playing    int     `json:"playing"`
	Finished   int     `json:"finished"`
	Total      int     `json:"total"`
	AvgScore   float64 `json:"avgScore"`
	AvgCorrect float64 `json:"avgCorrect"`
}

// Board is the admin live view.
type Board struct {
	Settings      model.GameSettings `json:"settings"`
	QuestionCount int64              `json:"questionCount"`
	EasyCount     int64              `json:"easyCount"`
	HardCount     int64              `json:"hardCount"`
	// MixedRounds is true when finished players played different rounds
	// (question count or time per question changed mid-competition) — then
	// comparing their correct answers is not fair, and the admin is warned.
	MixedRounds bool       `json:"mixedRounds"`
	Players     []BoardRow `json:"players"`
	Podium      []BoardRow `json:"podium"`
	Stats       BoardStats `json:"stats"`
	ServerNow   int64      `json:"serverNow"`
}

// RankFinished orders finished players: more correct answers first, then the
// shorter total answer time, then whoever finished earlier.
func RankFinished(rows []BoardRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Correct != b.Correct {
			return a.Correct > b.Correct
		}
		if a.TotalMs != b.TotalMs {
			return a.TotalMs < b.TotalMs
		}
		return a.FinishedAt < b.FinishedAt
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
}

// Board builds the ranked leaderboard: finished players by rank, then the ones
// still playing (most progress first), then registered-only.
func (s *GameService) Board() (*Board, error) {
	var players []model.GamePlayer
	if err := s.db.Omit("items", "token").Order("created_at ASC").Find(&players).Error; err != nil {
		return nil, err
	}
	var finished, playing, waiting []BoardRow
	st := BoardStats{Total: len(players)}
	type round struct {
		total int
		limit int64
	}
	rounds := map[round]bool{}
	var sumScore, sumCorrect int
	for _, p := range players {
		r := BoardRow{ID: p.ID, FirstName: p.FirstName, LastName: p.LastName, Phone: p.Phone, Status: p.Status,
			CreatedAt: p.CreatedAt, StartedAt: p.StartedAt, FinishedAt: p.FinishedAt, Correct: p.Correct,
			Answered: p.Answered, Total: p.Total, TotalMs: p.TotalMs, Score: p.Score}
		switch p.Status {
		case model.GameFinished:
			finished = append(finished, r)
			rounds[round{p.Total, p.LimitMs}] = true
			sumScore += p.Score
			sumCorrect += p.Correct
		case model.GamePlaying:
			playing = append(playing, r)
		default:
			waiting = append(waiting, r)
		}
	}
	RankFinished(finished)
	sort.SliceStable(playing, func(i, j int) bool { return playing[i].Answered > playing[j].Answered })
	sort.SliceStable(waiting, func(i, j int) bool { return waiting[i].CreatedAt > waiting[j].CreatedAt })

	st.Finished, st.Playing, st.Registered = len(finished), len(playing), len(waiting)
	if len(finished) > 0 {
		st.AvgScore = float64(sumScore) / float64(len(finished))
		st.AvgCorrect = float64(sumCorrect) / float64(len(finished))
	}

	rows := make([]BoardRow, 0, len(players))
	rows = append(rows, finished...)
	rows = append(rows, playing...)
	rows = append(rows, waiting...)
	// the podium only holds players who got at least one answer right
	podium := make([]BoardRow, 0, 3)
	for _, r := range finished {
		if len(podium) == 3 || r.Correct == 0 {
			break
		}
		podium = append(podium, r)
	}
	var easy, hard int64
	s.db.Model(&model.GameQuestion{}).Where("difficulty = ?", model.DifficultyHard).Count(&hard)
	total := s.QuestionCount()
	easy = total - hard
	return &Board{
		Settings:      s.Settings(),
		QuestionCount: total,
		EasyCount:     easy,
		HardCount:     hard,
		MixedRounds:   len(rounds) > 1,
		Players:       rows,
		Podium:        podium,
		Stats:         st,
		ServerNow:     s.nowMs(),
	}, nil
}

// PlayerReview returns one player's answered questions for the admin.
func (s *GameService) PlayerReview(id string) ([]GameReviewItem, error) {
	var p model.GamePlayer
	if err := s.db.Where("id = ?", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.NotFound("بازیکن پیدا نشد.")
		}
		return nil, err
	}
	items := loadItems(&p)
	out := reviewOf(items)
	// -2 marks a question the player has not answered yet (still in play)
	for i := range out {
		if !items[i].Answered {
			out[i].Choice = -2
		}
	}
	return out, nil
}

// DeletePlayer removes a player and their result.
func (s *GameService) DeletePlayer(id string) (bool, error) {
	mu := s.stripe(id)
	mu.Lock()
	defer mu.Unlock()
	res := s.db.Where("id = ?", id).Delete(&model.GamePlayer{})
	return res.RowsAffected > 0, res.Error
}

// Replay lets a player clear their own run and play again — only while the
// admin has test mode switched on.
func (s *GameService) Replay(token string) (*GameState, error) {
	if !s.Settings().TestMode {
		return nil, apperr.New(403, "GAME_REPLAY_DISABLED", "بازی دوباره فقط در حالت تست ممکن است.")
	}
	p, unlock, err := s.lockPlayer(token)
	if err != nil {
		return nil, err
	}
	err = s.resetLocked(p.ID)
	unlock()
	if err != nil {
		return nil, err
	}
	return s.State(token)
}

// ResetPlayer wipes a player's run so they can play again (keeps registration).
func (s *GameService) ResetPlayer(id string) error {
	mu := s.stripe(id)
	mu.Lock()
	defer mu.Unlock()
	return s.resetLocked(id)
}

func (s *GameService) resetLocked(id string) error {
	res := s.db.Model(&model.GamePlayer{}).Where("id = ?", id).Updates(map[string]any{
		"status": model.GameRegistered, "started_at": 0, "finished_at": 0, "limit_ms": 0, "current": 0,
		"correct": 0, "answered": 0, "total": 0, "total_ms": 0, "score": 0, "items": "[]",
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("بازیکن پیدا نشد.")
	}
	return nil
}

// DeleteAllPlayers clears every player and result (a fresh competition).
func (s *GameService) DeleteAllPlayers() (int64, error) {
	for i := range s.stripes {
		s.stripes[i].Lock()
	}
	defer func() {
		for i := range s.stripes {
			s.stripes[i].Unlock()
		}
	}()
	res := s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.GamePlayer{})
	return res.RowsAffected, res.Error
}
