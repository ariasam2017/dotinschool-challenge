package model

// GameQuestion is one four-option question in the quiz bank. Players are dealt
// a random subset; the correct option index is never sent to a player before
// they answer.
type GameQuestion struct {
	ID         string `gorm:"primaryKey;column:id" json:"id"`
	Category   string `gorm:"column:category;not null;default:''" json:"category"`
	Difficulty string `gorm:"column:difficulty;not null;default:easy" json:"difficulty"` // easy | hard
	Text       string `gorm:"column:text;not null" json:"text"`
	Option1    string `gorm:"column:option1;not null" json:"-"`
	Option2    string `gorm:"column:option2;not null" json:"-"`
	Option3    string `gorm:"column:option3;not null" json:"-"`
	Option4    string `gorm:"column:option4;not null" json:"-"`
	Correct    int    `gorm:"column:correct;not null" json:"correct"` // 0..3
	CreatedAt  int64  `gorm:"column:created_at;not null" json:"createdAt"`
}

func (GameQuestion) TableName() string { return "game_questions" }

// Question difficulty levels.
const (
	DifficultyEasy = "easy"
	DifficultyHard = "hard"
)

// DifficultyPlan is the easy/hard order of the questions in one game of n
// questions: blocks of ten go E E E H H E E E E H, then a block of five
// E E E H H, then a shorter tail (E · E H · E E H · E E E H). So a 5-question
// game is 3 easy then 2 hard, and a 10-question game is 3 easy, 2 hard,
// 4 easy, 1 hard.
func DifficultyPlan(n int) []string {
	const e, h = DifficultyEasy, DifficultyHard
	ten := []string{e, e, e, h, h, e, e, e, e, h}
	five := []string{e, e, e, h, h}
	tails := [][]string{{}, {e}, {e, h}, {e, e, h}, {e, e, e, h}}
	plan := make([]string, 0, n)
	for n-len(plan) >= 10 {
		plan = append(plan, ten...)
	}
	if n-len(plan) >= 5 {
		plan = append(plan, five...)
	}
	return append(plan, tails[n-len(plan)]...)
}

// Options returns the four answers in their authored order.
func (q GameQuestion) Options() [4]string {
	return [4]string{q.Option1, q.Option2, q.Option3, q.Option4}
}

// SetOptions stores the four answers.
func (q *GameQuestion) SetOptions(o [4]string) {
	q.Option1, q.Option2, q.Option3, q.Option4 = o[0], o[1], o[2], o[3]
}

// GamePlayer is one registered quiz player together with their single run.
// The game is individual and one-shot: a player is dealt Items at start and
// walks through them in order; the server times every answer.
type GamePlayer struct {
	ID         string `gorm:"primaryKey;column:id" json:"id"`
	Token      string `gorm:"uniqueIndex;column:token;not null" json:"-"`
	FirstName  string `gorm:"column:first_name;not null" json:"firstName"`
	LastName   string `gorm:"column:last_name;not null" json:"lastName"`
	Phone      string `gorm:"uniqueIndex;column:phone;not null" json:"phone"`
	CreatedAt  int64  `gorm:"column:created_at;not null" json:"createdAt"`
	Status     string `gorm:"column:status;not null;default:registered" json:"status"`
	StartedAt  int64  `gorm:"column:started_at;not null;default:0" json:"startedAt"`
	FinishedAt int64  `gorm:"column:finished_at;not null;default:0" json:"finishedAt"`
	LimitMs    int64  `gorm:"column:limit_ms;not null;default:0" json:"limitMs"` // per question, fixed at start
	Current    int    `gorm:"column:current;not null;default:0" json:"current"`
	Correct    int    `gorm:"column:correct;not null;default:0" json:"correct"`
	Answered   int    `gorm:"column:answered;not null;default:0" json:"answered"`
	Total      int    `gorm:"column:total;not null;default:0" json:"total"`
	TotalMs    int64  `gorm:"column:total_ms;not null;default:0" json:"totalMs"`
	Score      int    `gorm:"column:score;not null;default:0" json:"score"`
	// Items is the JSON-encoded []GameItem dealt at start.
	Items string `gorm:"column:items;not null;default:'[]'" json:"-"`
}

func (GamePlayer) TableName() string { return "game_players" }

// Game player status values.
const (
	GameRegistered = "registered"
	GamePlaying    = "playing"
	GameFinished   = "finished"
)

// GameItem is one dealt question, snapshotted (text + shuffled options) so a
// later edit or delete in the bank never changes a played game.
type GameItem struct {
	QID        string    `json:"qid"`
	Category   string    `json:"category"`
	Difficulty string    `json:"difficulty,omitempty"`
	Text       string    `json:"text"`
	Options    [4]string `json:"options"` // display order
	Correct    int       `json:"correct"` // display index
	ServedAt   int64     `json:"servedAt"`
	Answered   bool      `json:"answered"`
	Choice     int       `json:"choice"` // display index, -1 = time ran out
	IsCorrect  bool      `json:"isCorrect"`
	Ms         int64     `json:"ms"`
}

// GameSettings is the admin-tunable quiz configuration (settings key "game").
type GameSettings struct {
	Open               bool   `json:"open"`
	QuestionsPerGame   int    `json:"questionsPerGame"`
	SecondsPerQuestion int    `json:"secondsPerQuestion"`
	FinishMessage      string `json:"finishMessage"`

	// Winner-announcement animation: how long the name roller spins before
	// each place is revealed (3rd, then 2nd, then 1st), and the pause
	// between one reveal and the next roll — all in seconds.
	RevealThirdSec  int `json:"revealThirdSec"`
	RevealSecondSec int `json:"revealSecondSec"`
	RevealFirstSec  int `json:"revealFirstSec"`
	RevealGapSec    int `json:"revealGapSec"`

	// TestMode lets a player wipe their own finished run and play again from
	// the result screen — for rehearsing before the real competition.
	TestMode bool `json:"testMode"`

	// SocialDelaySec is how long a player sees their own result before the
	// "follow us" screen (logo + social links) takes over.
	SocialDelaySec int `json:"socialDelaySec"`

	// MixOn deals every game from Mix — a fixed number of easy and hard
	// questions per category — instead of QuestionsPerGame questions from the
	// whole bank. While it is on, QuestionsPerGame is the sum of Mix.
	MixOn bool      `json:"mixOn"`
	Mix   []GameMix `json:"mix"`
}

// GameMix is one category's share of a game.
type GameMix struct {
	Category string `json:"category"`
	Easy     int    `json:"easy"`
	Hard     int    `json:"hard"`
}

// DefaultGameMix is the competition recipe: 10 questions = 6 programming
// (3 easy + 3 hard) and 4 technology (2 easy + 2 hard).
func DefaultGameMix() []GameMix {
	return []GameMix{
		{Category: ProgrammingGameCategory, Easy: 3, Hard: 3},
		{Category: DefaultGameCategory, Easy: 2, Hard: 2},
	}
}

// MixTotal is the number of questions one game of this mix has.
func MixTotal(mix []GameMix) int {
	n := 0
	for _, m := range mix {
		n += m.Easy + m.Hard
	}
	return n
}

// GameSlot is one question position of a game: which category and level it
// is drawn from. An empty Category means "any category".
type GameSlot struct {
	Category   string
	Difficulty string
}

// MixPlan is the question order of a mixed game: every easy question first,
// then every hard one, and inside each level the categories take turns in
// Mix order. The default mix gives
// P·E T·E P·E T·E P·E  P·H T·H P·H T·H P·H (P = programming, T = technology).
func MixPlan(mix []GameMix) []GameSlot {
	plan := make([]GameSlot, 0, MixTotal(mix))
	for _, lv := range []string{DifficultyEasy, DifficultyHard} {
		left := make([]int, len(mix))
		for i, m := range mix {
			left[i] = m.Easy
			if lv == DifficultyHard {
				left[i] = m.Hard
			}
		}
		for more := true; more; {
			more = false
			for i, m := range mix {
				if left[i] > 0 {
					plan = append(plan, GameSlot{Category: m.Category, Difficulty: lv})
					left[i]--
					more = true
				}
			}
		}
	}
	return plan
}

// GamePlan is the slot list of one game under these settings: the mix when
// it is on, otherwise DifficultyPlan(QuestionsPerGame) over every category.
func (gs GameSettings) GamePlan() []GameSlot {
	if gs.MixOn && MixTotal(gs.Mix) > 0 {
		return MixPlan(gs.Mix)
	}
	levels := DifficultyPlan(gs.QuestionsPerGame)
	plan := make([]GameSlot, len(levels))
	for i, lv := range levels {
		plan[i] = GameSlot{Difficulty: lv}
	}
	return plan
}

// KeyGameSettings stores GameSettings.
const KeyGameSettings = "game"

// DefaultGameSettings: 20 s per question, and the 10-question category mix
// (QuestionsPerGame 3 is the count used once the admin turns the mix off).
func DefaultGameSettings() GameSettings {
	return GameSettings{
		Open:               true,
		QuestionsPerGame:   3,
		SecondsPerQuestion: 20,
		FinishMessage:      "بازی‌ات تمام شد و نتیجه ثبت شد! برندگان پس از پایان مسابقه اعلام می‌شوند. ممنون که شرکت کردی 🌟",
		RevealThirdSec:     6,
		RevealSecondSec:    9,
		RevealFirstSec:     15,
		RevealGapSec:       4,
		SocialDelaySec:     8,
		MixOn:              true,
		Mix:                DefaultGameMix(),
	}
}
