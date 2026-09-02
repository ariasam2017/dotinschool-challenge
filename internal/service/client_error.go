package service

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/model"
)

// ClientErrorService stores failed-submission reports from entrants' browsers.
type ClientErrorService struct {
	db *gorm.DB
}

func newClientErrorService(db *gorm.DB) *ClientErrorService {
	return &ClientErrorService{db: db}
}

// Record inserts one report and trims the table back to the newest 500 rows.
func (s *ClientErrorService) Record(e model.ClientError) error {
	e.ID = uuid.NewString()
	e.CreatedAt = time.Now().UnixMilli()
	e.Seen = false
	if err := s.db.Create(&e).Error; err != nil {
		return err
	}
	// Keep only the most recent MaxClientErrors rows.
	s.db.Exec(`DELETE FROM client_errors WHERE id NOT IN (
		SELECT id FROM client_errors ORDER BY created_at DESC LIMIT ?
	)`, model.MaxClientErrors)
	return nil
}

// ClientErrorList is the admin payload.
type ClientErrorList struct {
	Unseen int64               `json:"unseen"`
	Errors []model.ClientError `json:"errors"`
}

// List returns the newest 300 reports plus the unseen count.
func (s *ClientErrorService) List() (ClientErrorList, error) {
	var out ClientErrorList
	if err := s.db.Order("created_at DESC").Limit(300).Find(&out.Errors).Error; err != nil {
		return out, err
	}
	s.db.Model(&model.ClientError{}).Where("seen = ?", false).Count(&out.Unseen)
	return out, nil
}

// MarkAllSeen clears the unseen badge.
func (s *ClientErrorService) MarkAllSeen() error {
	return s.db.Model(&model.ClientError{}).Where("seen = ?", false).Update("seen", true).Error
}

// DeleteOne removes a single report. Returns false when nothing matched.
func (s *ClientErrorService) DeleteOne(id string) (bool, error) {
	res := s.db.Where("id = ?", id).Delete(&model.ClientError{})
	return res.RowsAffected > 0, res.Error
}

// DeleteAll clears the whole list.
func (s *ClientErrorService) DeleteAll() error {
	return s.db.Where("1 = 1").Delete(&model.ClientError{}).Error
}
