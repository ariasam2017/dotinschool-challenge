// Command seed wipes and repopulates the database with a lively demo dataset:
// ~16 participants across every status, a running 90-minute event, and a
// published challenge. For quick demos and screenshots — never run it against
// real event data.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool/internal/config"
	"github.com/dotinschool/dotinschool/internal/database"
	"github.com/dotinschool/dotinschool/internal/model"
)

func main() {
	log := logrus.New()
	log.SetLevel(logrus.WarnLevel)

	wd, _ := os.Getwd()
	cfg := config.Load(wd, log)
	db, err := database.Open(cfg, log)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close(db)

	if err := db.Exec("DELETE FROM participants").Error; err != nil {
		log.Fatalf("wipe: %v", err)
	}

	now := time.Now().UnixMilli()
	rng := rand.New(rand.NewSource(42))

	names := [][2]string{
		{"علی", "رضایی"}, {"سارا", "محمدی"}, {"امیر", "حسینی"}, {"نگار", "کریمی"},
		{"مهدی", "صادقی"}, {"زهرا", "احمدی"}, {"رضا", "قاسمی"}, {"یاسمن", "نوری"},
		{"حسین", "رحیمی"}, {"فاطمه", "عباسی"}, {"محمد", "کاظمی"}, {"الهام", "موسوی"},
		{"پارسا", "اکبری"}, {"مریم", "جعفری"}, {"کیان", "شریفی"}, {"نیلوفر", "بهرامی"},
	}
	statuses := []string{
		model.StatusSubmitted, model.StatusSubmitted, model.StatusStarted, model.StatusSubmitted,
		model.StatusStarted, model.StatusRegistered, model.StatusSubmitted, model.StatusStarted,
		model.StatusRegistered, model.StatusSubmitted, model.StatusSubmitted, model.StatusStarted,
		model.StatusRegistered, model.StatusSubmitted, model.StatusStarted, model.StatusRegistered,
	}

	cleanCode := `import java.util.*;

public class Main {
    public int[] twoSum(int[] nums, int target) {
        Map<Integer, Integer> seen = new HashMap<>();
        for (int i = 0; i < nums.length; i++) {
            int need = target - nums[i];
            if (seen.containsKey(need)) return new int[] { seen.get(need), i };
            seen.put(nums[i], i);
        }
        return new int[0];
    }
}`
	messyCode := `public class Main {
    public int[] twoSum(int[] nums, int target) {
        for (int i = 0; i < nums.length; i++)
            for (int j = i + 1; j < nums.length; j++)
                if (nums[i] + nums[j] == target)
                    return new int[] { i, j };
        return new int[0];
    }
}`

	intPtr := func(v int) *int { return &v }
	strPtr := func(v string) *string { return &v }

	for i, n := range names {
		st := statuses[i]
		p := model.Participant{
			ID:           uuid.NewString(),
			PublicID:     fmt.Sprintf("dtn-%04d", i+1),
			FirstName:    n[0],
			LastName:     n[1],
			Phone:        fmt.Sprintf("912%07d", 1000000+rng.Intn(8999999)),
			RegisteredAt: now - int64((len(names)-i))*7*60000 - int64(rng.Intn(120000)),
			Status:       st,
		}
		if st == model.StatusSubmitted {
			p.Score = intPtr(45 + rng.Intn(55))
			if i%2 == 0 {
				p.Code = strPtr(cleanCode)
				p.ExecMs = intPtr(18 + rng.Intn(40))
				p.ExecDepth = intPtr(1)
				if rng.Intn(3) == 0 {
					p.AIScore = intPtr(8 + rng.Intn(22))
					p.AIFlag = strPtr(model.AIFlagHuman)
				}
			} else {
				p.Code = strPtr(messyCode)
				p.ExecMs = intPtr(120 + rng.Intn(400))
				p.ExecDepth = intPtr(2)
				if rng.Intn(2) == 0 {
					p.AIScore = intPtr(55 + rng.Intn(40))
					p.AIFlag = strPtr(model.AIFlagSuspicious)
				}
			}
		}
		if err := db.Create(&p).Error; err != nil {
			log.Fatalf("insert %s: %v", p.PublicID, err)
		}
	}

	// a running 90-minute event with a fresh challenge
	end := now + 90*60000
	set := func(k string, v any) {
		raw, _ := json.Marshal(v)
		db.Save(&model.Setting{Key: k, Value: string(raw)})
	}
	set(model.KeyEventEndsAt, end)
	set(model.KeyEventStartsAt, nil)
	set(model.KeyEventPaused, false)
	set(model.KeyPausedRemainMs, nil)
	ch := model.DefaultChallenge()
	ch.Mode = model.ModePerformance
	ch.TimeLimitMs = 150
	ch.Tag = "آرایه‌ها، هش و پیچیدگی"
	set(model.KeyChallenge, ch)

	fmt.Printf("✓ %d شرکت‌کننده‌ی دمو ساخته شد. رویداد زنده تا %s.\n",
		len(names), time.UnixMilli(end).Format("15:04"))
}
