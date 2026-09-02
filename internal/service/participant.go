package service

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/model"
)

// ParticipantService owns persistence for entrants. Authorisation rules
// (admin vs. session-owner, field allow-lists, the live-event lock) are
// enforced in the HTTP handler, not here.
type ParticipantService struct {
	db *gorm.DB
}

func newParticipantService(db *gorm.DB) *ParticipantService {
	return &ParticipantService{db: db}
}

// List returns every participant ordered by registration time ascending.
func (s *ParticipantService) List() ([]model.Participant, error) {
	var out []model.Participant
	err := s.db.Order("registered_at ASC").Find(&out).Error
	return out, err
}

// GetByID fetches one participant; returns gorm.ErrRecordNotFound when absent.
func (s *ParticipantService) GetByID(id string) (*model.Participant, error) {
	var p model.Participant
	if err := s.db.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByPhone fetches one participant by phone.
func (s *ParticipantService) GetByPhone(phone string) (*model.Participant, error) {
	var p model.Participant
	if err := s.db.Where("phone = ?", phone).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ExistsByPhone reports whether a participant with this phone is registered.
func (s *ParticipantService) ExistsByPhone(phone string) bool {
	var count int64
	s.db.Model(&model.Participant{}).Where("phone = ?", phone).Count(&count)
	return count > 0
}

var publicIDNum = regexp.MustCompile(`^dtn-(\d+)$`)

// NextPublicID returns the next `dtn-0001` style id. It is based on the highest
// number ever issued (not COUNT), so deleting an entrant never lets a future id
// collide with a live one — the behaviour of a real auto-increment column.
func (s *ParticipantService) NextPublicID() string {
	var row model.Participant
	err := s.db.
		Where("public_id GLOB ?", "dtn-*").
		Order("public_id DESC").
		Limit(1).
		First(&row).Error

	seq := 1
	if err == nil {
		if m := publicIDNum.FindStringSubmatch(row.PublicID); m != nil {
			if n, perr := strconv.Atoi(m[1]); perr == nil {
				seq = n + 1
			}
		}
	}
	return fmt.Sprintf("dtn-%04d", seq)
}

// Create inserts a new entrant in the "registered" state. telegram is stored
// empty for now, exactly as the original does.
func (s *ParticipantService) Create(firstName, lastName, phone string) (*model.Participant, error) {
	p := &model.Participant{
		ID:           uuid.NewString(),
		PublicID:     s.NextPublicID(),
		FirstName:    firstName,
		LastName:     lastName,
		Phone:        phone,
		Telegram:     "",
		RegisteredAt: time.Now().UnixMilli(),
		Status:       model.StatusRegistered,
	}
	if err := s.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// Save persists an already-mutated participant record in full.
func (s *ParticipantService) Save(p *model.Participant) error {
	return s.db.Save(p).Error
}

// Delete removes a participant. Returns (false, nil) when nothing matched.
func (s *ParticipantService) Delete(id string) (bool, error) {
	res := s.db.Where("id = ?", id).Delete(&model.Participant{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
