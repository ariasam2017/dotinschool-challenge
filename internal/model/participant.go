package model

// Participant is one registered entrant. Nullable columns use pointers so the
// wire format can distinguish "not set" (JSON null) from a zero value, matching
// the original `rowToParticipant` shape exactly.
type Participant struct {
	ID           string  `gorm:"primaryKey;column:id" json:"id"`
	PublicID     string  `gorm:"uniqueIndex;column:public_id;not null" json:"publicId"`
	FirstName    string  `gorm:"column:first_name;not null" json:"firstName"`
	LastName     string  `gorm:"column:last_name;not null" json:"lastName"`
	Phone        string  `gorm:"uniqueIndex;column:phone;not null" json:"phone"`
	Telegram     string  `gorm:"column:telegram;not null;default:''" json:"telegram"`
	RegisteredAt int64   `gorm:"column:registered_at;not null" json:"registeredAt"` // epoch ms
	Status       string  `gorm:"column:status;not null;default:registered" json:"status"`
	Score        *int    `gorm:"column:score" json:"score"`
	Code         *string `gorm:"column:code" json:"code"`
	AIFlag       *string `gorm:"column:ai_flag" json:"aiFlag"`
	ExecMs       *int    `gorm:"column:exec_ms" json:"execMs"`
	ExecDepth    *int    `gorm:"column:exec_depth" json:"execDepth"`
	AIScore      *int    `gorm:"column:ai_score" json:"aiScore"`
}

// TableName pins the table name regardless of GORM's pluralizer.
func (Participant) TableName() string { return "participants" }

// Participant status values.
const (
	StatusRegistered = "registered"
	StatusStarted    = "started"
	StatusSubmitted  = "submitted"
)

// AI flag values.
const (
	AIFlagHuman      = "human"
	AIFlagSuspicious = "suspicious"
)
