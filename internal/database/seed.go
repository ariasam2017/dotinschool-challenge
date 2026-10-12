package database

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/dotinschool/dotinschool/internal/model"
)

// defaultAppearance ships the project's baseline Design Studio layer. It is
// "{}" by default; replace its contents with a panel «خروجی» export to make a
// tuned design the shipped default for fresh installs.
//
//go:embed appearance.default.json
var defaultAppearance []byte

// nowMs is the current time in epoch milliseconds.
func nowMs() int64 { return time.Now().UnixMilli() }

// EnsureDefaults inserts the config rows a fresh install needs. A brand-new
// database starts *paused*: until the admin explicitly schedules or starts the
// event, nobody can write or submit code.
func EnsureDefaults(db *gorm.DB) error {
	const day = int64(1440 * 60000)

	set := func(key string, v any) error {
		var count int64
		if err := db.Model(&model.Setting{}).Where("key = ?", key).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return db.Create(&model.Setting{Key: key, Value: string(raw)}).Error
	}

	if err := set(model.KeyEventEndsAt, nowMs()+day); err != nil {
		return err
	}
	if err := set(model.KeyChallenge, model.DefaultChallenge()); err != nil {
		return err
	}
	if err := set(model.KeyEventPaused, true); err != nil {
		return err
	}
	if err := set(model.KeyPausedRemainMs, day); err != nil {
		return err
	}
	if err := set(model.KeyEventStartsAt, nil); err != nil {
		return err
	}
	if err := set(model.KeyGameSettings, model.DefaultGameSettings()); err != nil {
		return err
	}
	if err := seedGameBank(db); err != nil {
		return err
	}

	// Ship a tuned Design Studio layer as the default for a fresh install
	// (no-op while appearance.default.json is just "{}").
	if trimmed := strings.TrimSpace(string(defaultAppearance)); len(trimmed) > 2 {
		var a model.Appearance
		if json.Unmarshal(defaultAppearance, &a) == nil {
			if err := set(model.KeyAppearancePublished, a); err != nil {
				return err
			}
			if err := set(model.KeyAppearanceDraft, a); err != nil {
				return err
			}
		}
	}
	return nil
}

// keyGameBankSeeded is the legacy "bank loaded once" flag (= version 1);
// keyGameBankVersion records which built-in bank version this DB has received.
const (
	keyGameBankSeeded  = "gameBankSeeded"
	keyGameBankVersion = "gameBankVersion"
)

// seedGameBank adds the built-in technology/programming questions this database has not
// received yet. It runs once per bank version (model.GameBankVersion), so an
// admin who deletes questions does not get them back on every restart — only
// when a newer built-in bank ships, and then only questions whose text is not
// already present.
func seedGameBank(db *gorm.DB) error {
	have := 0
	var row model.Setting
	if db.Where("key = ?", keyGameBankVersion).First(&row).Error == nil {
		_ = json.Unmarshal([]byte(row.Value), &have)
	} else {
		var legacy int64
		if err := db.Model(&model.Setting{}).Where("key = ?", keyGameBankSeeded).Count(&legacy).Error; err != nil {
			return err
		}
		if legacy > 0 {
			have = 1
		}
	}
	if have >= model.GameBankVersion {
		return nil
	}

	if err := db.Where("text IN ?", model.RetiredBankTexts).Delete(&model.GameQuestion{}).Error; err != nil {
		return err
	}
	if have < 7 {
		if err := cleanFaQuestions(db); err != nil {
			return err
		}
	}
	var texts []string
	if err := db.Model(&model.GameQuestion{}).Pluck("text", &texts).Error; err != nil {
		return err
	}
	existing := make(map[string]bool, len(texts))
	for _, t := range texts {
		existing[t] = true
	}
	base := nowMs()
	var rows []model.GameQuestion
	for i, q := range model.DefaultGameQuestions() {
		if existing[q.Text] {
			continue
		}
		q.ID = uuid.NewString()
		q.CreatedAt = base - int64(i)
		rows = append(rows, q)
	}
	if len(rows) > 0 {
		if err := db.CreateInBatches(rows, 200).Error; err != nil {
			return err
		}
	}
	// v4: built-in questions carry a difficulty; tag the ones already in an
	// older database (matched by exact text — admin-written ones stay "easy")
	var hardTexts []string
	for _, q := range model.DefaultGameQuestions() {
		if q.Difficulty == model.DifficultyHard {
			hardTexts = append(hardTexts, q.Text)
		}
	}
	if err := db.Model(&model.GameQuestion{}).Where("text IN ?", hardTexts).
		Update("difficulty", model.DifficultyHard).Error; err != nil {
		return err
	}
	raw, _ := json.Marshal(model.GameBankVersion)
	return db.Save(&model.Setting{Key: keyGameBankVersion, Value: string(raw)}).Error
}

// cleanFaQuestions (v7) rewrites every stored question — built-in and
// admin-written — with model.CleanFaText, in place, so ids and played results
// stay valid. A question whose options would collide after cleaning is left
// as is rather than made ambiguous.
func cleanFaQuestions(db *gorm.DB) error {
	var rows []model.GameQuestion
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, q := range rows {
			if !model.CleanFaQuestion(&q) {
				continue
			}
			opts := q.Options()
			seen := map[string]bool{}
			dup := false
			for _, o := range opts {
				dup = dup || seen[o]
				seen[o] = true
			}
			if dup {
				continue
			}
			if err := tx.Model(&model.GameQuestion{}).Where("id = ?", q.ID).Updates(map[string]any{
				"text": q.Text, "option1": opts[0], "option2": opts[1], "option3": opts[2], "option4": opts[3],
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

var namesPool = [][2]string{
	{"علی", "رضایی"}, {"سارا", "محمدی"}, {"امیر", "حسینی"}, {"نگار", "کریمی"}, {"مهدی", "صادقی"},
	{"زهرا", "احمدی"}, {"رضا", "قاسمی"}, {"یاسمن", "نوری"}, {"حسین", "رحیمی"}, {"فاطمه", "عباسی"},
}

const sampleCodeClean = `public class Solution {
    public int[] twoSum(int[] nums, int target) {
        Map<Integer, Integer> seen = new HashMap<>();
        for (int i = 0; i < nums.length; i++) {
            int need = target - nums[i];
            if (seen.containsKey(need)) {
                return new int[] { seen.get(need), i };
            }
            seen.put(nums[i], i);
        }
        return new int[0];
    }
}`

const sampleCodeMessy = `public class Solution {
    public int[] twoSum(int[] nums, int target) {
        for (int i = 0; i < nums.length; i++) {
            for (int j = i + 1; j < nums.length; j++) {
                if (nums[i] + nums[j] == target) {
                    return new int[] { i, j }
                }
            }
        }
        return new int[0]
    }
}`

// Seed populates a handful of demo participants, but only when the table is
// empty — so it never fights real data.
func Seed(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.Participant{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	statuses := []string{
		model.StatusRegistered, model.StatusStarted, model.StatusSubmitted,
		model.StatusSubmitted, model.StatusStarted, model.StatusRegistered,
	}

	seq := 0
	nextPublicID := func() string {
		seq++
		return fmt.Sprintf("dtn-%04d", seq)
	}
	intPtr := func(v int) *int { return &v }
	strPtr := func(v string) *string { return &v }

	for i := 0; i < 6; i++ {
		f, l := namesPool[i][0], namesPool[i][1]
		st := statuses[i]
		p := model.Participant{
			ID:           uuid.NewString(),
			PublicID:     nextPublicID(),
			FirstName:    f,
			LastName:     l,
			Phone:        fmt.Sprintf("9%09d", rng.Intn(1_000_000_000)),
			Telegram:     "",
			RegisteredAt: nowMs() - int64(rng.Intn(5*3600000)),
			Status:       st,
		}
		if st == model.StatusSubmitted {
			p.Score = intPtr(40 + rng.Intn(60))
			if i%2 == 0 {
				p.Code = strPtr(sampleCodeClean)
			} else {
				p.Code = strPtr(sampleCodeMessy)
			}
		}
		if err := db.Create(&p).Error; err != nil {
			return err
		}
	}

	known := model.Participant{
		ID:           uuid.NewString(),
		PublicID:     nextPublicID(),
		FirstName:    "محمد",
		LastName:     "کاظمی",
		Phone:        "9121234567",
		Telegram:     "",
		RegisteredAt: nowMs() - 7200000,
		Status:       model.StatusSubmitted,
		Score:        intPtr(71),
		Code:         strPtr(sampleCodeMessy),
	}
	return db.Create(&known).Error
}
