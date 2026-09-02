package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/model"
)

func memDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.Participant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// The public id must advance from the highest number ever issued, not COUNT —
// so deleting an entrant never lets a future id collide with a live one.
func TestNextPublicID_MaxBasedNotCount(t *testing.T) {
	db := memDB(t)
	svc := newParticipantService(db)

	if got := svc.NextPublicID(); got != "dtn-0001" {
		t.Fatalf("first id = %q, want dtn-0001", got)
	}

	for i := 1; i <= 5; i++ {
		if _, err := svc.Create("n", "n", string(rune('0'+i))+"000000000"); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	// now dtn-0001..dtn-0005 exist; delete the middle three
	db.Where("public_id IN ?", []string{"dtn-0002", "dtn-0003", "dtn-0004"}).Delete(&model.Participant{})

	if got := svc.NextPublicID(); got != "dtn-0006" {
		t.Fatalf("after deletes id = %q, want dtn-0006 (max+1, not count+1)", got)
	}
}
