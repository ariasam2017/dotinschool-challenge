package model

// ClientError is one failed-submission report sent by an entrant's browser so
// the admin can see who got stuck, when, and with what message. Only the most
// recent 500 rows are kept.
type ClientError struct {
	ID            string `gorm:"primaryKey;column:id" json:"id"`
	CreatedAt     int64  `gorm:"column:created_at;not null;index" json:"createdAt"` // epoch ms
	ParticipantID string `gorm:"column:participant_id" json:"participantId"`
	PublicID      string `gorm:"column:public_id" json:"publicId"`
	Name          string `gorm:"column:name" json:"name"`
	Context       string `gorm:"column:context" json:"context"`
	ErrorCode     string `gorm:"column:error_code" json:"errorCode"`
	ErrorMessage  string `gorm:"column:error_message" json:"errorMessage"`
	CodeExcerpt   string `gorm:"column:code_excerpt" json:"codeExcerpt"`
	UserAgent     string `gorm:"column:user_agent" json:"userAgent"`
	Seen          bool   `gorm:"column:seen;not null;default:false" json:"seen"`
}

func (ClientError) TableName() string { return "client_errors" }

// MaxClientErrors is the retention cap enforced after every insert.
const MaxClientErrors = 500
