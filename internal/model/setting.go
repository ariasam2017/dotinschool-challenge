package model

// Setting is one key/value row. Value holds a JSON-encoded payload (a number,
// a bool, null, or an object) — the same shape the original `config` table used.
type Setting struct {
	Key   string `gorm:"primaryKey;column:key" json:"key"`
	Value string `gorm:"column:value;not null" json:"value"`
}

func (Setting) TableName() string { return "settings" }

// Known setting keys.
const (
	KeyEventEndsAt    = "eventEndsAt"
	KeyEventStartsAt  = "eventStartsAt"
	KeyEventPaused    = "eventPaused"
	KeyPausedRemainMs = "pausedRemainMs"
	KeyChallenge      = "challenge"
	KeyAISettings     = "aiSettings"
)

// Challenge is the problem statement published to entrants.
type Challenge struct {
	Title         string  `json:"title"`
	Diff          string  `json:"diff"` // easy | medium | hard
	Tag           string  `json:"tag"`
	Desc          string  `json:"desc"`
	SampleIn      string  `json:"sampleIn"`
	SampleOut     string  `json:"sampleOut"`
	Image         *string `json:"image"` // data URL or null
	Mode          string  `json:"mode"`  // correctness | performance
	TimeLimitMs   int     `json:"timeLimitMs"`
	SubmitMessage string  `json:"submitMessage"`
}

// Challenge modes and difficulty values.
const (
	ModeCorrectness = "correctness"
	ModePerformance = "performance"

	DiffEasy   = "easy"
	DiffMedium = "medium"
	DiffHard   = "hard"
)

// DefaultSubmitMessage is shown to an entrant after a successful submission,
// in place of any score. The admin can override it from the challenge editor.
const DefaultSubmitMessage = "کدت با موفقیت ثبت شد. پس از بررسی مدیر سامانه، نتیجه‌ی چالش به شما اعلام خواهد شد."

// DefaultChallenge seeds a fresh database.
func DefaultChallenge() Challenge {
	return Challenge{
		Title:         "جمع دوتایی — Two Sum",
		Diff:          DiffMedium,
		Tag:           "آرایه‌ها و هش",
		Desc:          "یک آرایه از اعداد صحیح به نام nums و یک عدد هدف به نام target در اختیار دارید. اندیس دو عضوی از آرایه را برگردانید که مجموع آن‌ها برابر با target باشد.\n\nفرض کنید هر ورودی دقیقاً یک پاسخ دارد و نمی‌توانید از یک عضو دوبار استفاده کنید.",
		SampleIn:      "nums = [2, 7, 11, 15], target = 9",
		SampleOut:     "[0, 1]",
		Image:         nil,
		Mode:          ModeCorrectness,
		TimeLimitMs:   200,
		SubmitMessage: DefaultSubmitMessage,
	}
}

// AISettings is the panel-managed provider list plus the active selection.
type AISettings struct {
	ActiveID  string       `json:"activeId"`
	Providers []AIProvider `json:"providers"`
}

// AIProvider is one OpenAI-compatible endpoint.
type AIProvider struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	APIKey  string `json:"apiKey"`
}
