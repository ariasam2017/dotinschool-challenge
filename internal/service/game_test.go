package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/dotinschool/dotinschool/internal/model"
)

func gameSvc(t *testing.T) (*GameService, *time.Time) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // "record not found" is routine here
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// one connection, exactly like production (database.openFile)
	if sqlDB, derr := db.DB(); derr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.Setting{}, &model.GameQuestion{}, &model.GamePlayer{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g := newGameService(db, newSettingService(nil, db, logrus.New()))
	clock := time.UnixMilli(1_700_000_000_000)
	g.now = func() time.Time { return clock }
	if _, err := g.AddDefaultBank(); err != nil {
		t.Fatalf("bank: %v", err)
	}
	return g, &clock
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"09121234567":      "9121234567",
		"9121234567":       "9121234567",
		"+98 912 123 4567": "9121234567",
		"00989121234567":   "9121234567",
		"۰۹۱۲۱۲۳۴۵۶۷":      "9121234567",
		"0212345678":       "",
		"12345":            "",
	}
	for in, want := range cases {
		if got := NormalizePhone(in); got != want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGameScoreOrdering(t *testing.T) {
	limit := int64(20_000)
	// more correct answers always beat a faster run with fewer
	slow3 := ComputeGameScore(3, 3, 3*limit, limit)
	fast2 := ComputeGameScore(2, 3, 1, limit)
	if slow3 <= fast2 {
		t.Fatalf("3 correct slow (%d) must beat 2 correct fast (%d)", slow3, fast2)
	}
	// among equals, faster wins
	if ComputeGameScore(2, 3, 10_000, limit) <= ComputeGameScore(2, 3, 30_000, limit) {
		t.Fatal("faster run with equal correct answers must score higher")
	}
	if ComputeGameScore(0, 3, 1, limit) != 0 {
		t.Fatal("no correct answers scores 0")
	}
}

func TestGameFullRun(t *testing.T) {
	g, clock := gameSvc(t)
	p, created, err := g.Register("علی", "رضایی", "09121234567")
	if err != nil || !created {
		t.Fatalf("register: %v created=%v", err, created)
	}
	// same person again (new device) signs back in; a different name is refused
	if again, created, err := g.Register(" علی ", "رضایی", "9121234567"); err != nil || created || again.ID != p.ID {
		t.Fatalf("re-register: %v created=%v", err, created)
	}
	if _, _, err := g.Register("رضا", "محمدی", "9121234567"); err == nil {
		t.Fatal("a different name on a taken phone must be refused")
	}

	st, err := g.Start(p.Token)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if st.Phase != "question" || st.Total != 3 || st.Question == nil || st.Reveal != nil {
		t.Fatalf("unexpected start state: %+v", st)
	}

	// Q1: answer correctly after 4s. The key is only known server-side.
	items := func() []model.GameItem {
		var pl model.GamePlayer
		g.db.Where("id = ?", p.ID).First(&pl)
		return loadItems(&pl)
	}
	*clock = clock.Add(4 * time.Second)
	st, _ = g.Answer(p.Token, 0, items()[0].Correct)
	if st.Phase != "reveal" || !st.Reveal.IsCorrect || items()[0].Ms != 4000 {
		t.Fatalf("q1 reveal: %+v", st.Reveal)
	}
	// double tap on the same question is a harmless no-op
	st, _ = g.Answer(p.Token, 0, (items()[0].Correct+1)%4)
	if !st.Reveal.IsCorrect {
		t.Fatal("re-answer must not overwrite the first answer")
	}

	// Q2: wrong answer
	st, _ = g.Next(p.Token)
	if st.Phase != "question" || st.Index != 1 {
		t.Fatalf("next: %+v", st)
	}
	*clock = clock.Add(2 * time.Second)
	st, _ = g.Answer(p.Token, 1, (items()[1].Correct+1)%4)
	if st.Reveal.IsCorrect || st.Reveal.Correct != items()[1].Correct {
		t.Fatalf("q2 reveal: %+v", st.Reveal)
	}

	// Q3: player walks away — the deadline passes and State records a timeout
	g.Next(p.Token)
	*clock = clock.Add(60 * time.Second)
	st, _ = g.State(p.Token)
	if st.Phase != "finished" || st.Result == nil {
		t.Fatalf("expected finished, got %+v", st)
	}
	if st.Result.Correct != 1 {
		t.Fatalf("result: %+v", st.Result)
	}
	if st.Review[0].Ms != 0 {
		t.Fatal("per-question time must not reach the player")
	}
	if len(st.Review) != 3 || st.Review[2].Choice != -1 {
		t.Fatalf("review: %+v", st.Review)
	}

	// replay is refused unless the admin turned test mode on
	if _, err := g.Replay(p.Token); err == nil {
		t.Fatal("replay must be refused outside test mode")
	}

	b, err := g.Board()
	if err != nil || len(b.Podium) != 1 || b.Podium[0].Rank != 1 || b.Stats.Finished != 1 {
		t.Fatalf("board: %v %+v", err, b)
	}
	if b.Podium[0].TotalMs != 4000+2000+20000 {
		t.Fatalf("admin total time = %d", b.Podium[0].TotalMs)
	}
}

func TestGameReplayInTestMode(t *testing.T) {
	g, _ := gameSvc(t)
	gs := g.Settings()
	gs.TestMode = true
	gs.QuestionsPerGame = 1
	if _, err := g.SaveSettings(gs); err != nil {
		t.Fatal(err)
	}
	p, _, _ := g.Register("a", "b", "9120000009")
	g.Start(p.Token)
	st, _ := g.Answer(p.Token, 0, 0)
	if st.Phase != "finished" || !st.TestMode {
		t.Fatalf("want finished in test mode, got %+v", st)
	}
	st, err := g.Replay(p.Token)
	if err != nil || st.Phase != "ready" {
		t.Fatalf("replay: %v %+v", err, st)
	}
	if st, _ = g.Start(p.Token); st.Phase != "question" {
		t.Fatalf("second game did not start: %+v", st)
	}
}

func TestGameClosedAndRanking(t *testing.T) {
	g, clock := gameSvc(t)
	gs := g.Settings()
	if gs.RevealFirstSec != 15 || gs.RevealGapSec != 4 {
		t.Fatalf("reveal defaults not filled: %+v", gs)
	}
	gs.Open = false
	gs.QuestionsPerGame = 1
	if _, err := g.SaveSettings(gs); err != nil {
		t.Fatal(err)
	}
	p, _, _ := g.Register("a", "b", "9120000001")
	if _, err := g.Start(p.Token); err == nil {
		t.Fatal("start must fail while the game is closed")
	}
	gs.Open = true
	g.SaveSettings(gs)

	play := func(phone string, secs int, right bool) {
		pl, _, err := g.Register("n"+phone, "f", phone)
		if err != nil {
			t.Fatal(err)
		}
		g.Start(pl.Token)
		var row model.GamePlayer
		g.db.Where("id = ?", pl.ID).First(&row)
		it := loadItems(&row)[0]
		choice := it.Correct
		if !right {
			choice = (choice + 1) % 4
		}
		*clock = clock.Add(time.Duration(secs) * time.Second)
		g.Answer(pl.Token, 0, choice)
	}
	play("9120000002", 9, true)  // correct, slow
	play("9120000003", 2, false) // wrong, fast
	play("9120000004", 3, true)  // correct, fast  -> 1st

	b, _ := g.Board()
	// a player with no correct answer is ranked but never put on the podium
	if len(b.Podium) != 2 {
		t.Fatalf("podium size %d, want 2", len(b.Podium))
	}
	if b.Podium[0].Phone != "9120000004" || b.Podium[1].Phone != "9120000002" {
		t.Fatalf("wrong podium: %+v", b.Podium)
	}
	if b.Players[2].Phone != "9120000003" || b.Players[2].Rank != 3 {
		t.Fatalf("wrong ranking: %+v", b.Players)
	}
}

// The decks deal distinct questions to consecutive players until a difficulty
// level is used up, instead of independent random picks that overlap early.
func TestGameDeckSpreadsQuestions(t *testing.T) {
	g, _ := gameSvc(t)
	per := g.Settings().QuestionsPerGame
	need := map[string]int{}
	for _, lv := range model.DifficultyPlan(per) {
		need[lv]++
	}
	var hard int64
	g.db.Model(&model.GameQuestion{}).Where("difficulty = ?", model.DifficultyHard).Count(&hard)
	easy := g.QuestionCount() - hard
	players := int(easy) / need[model.DifficultyEasy]
	if h := int(hard) / need[model.DifficultyHard]; h < players {
		players = h
	}
	if players < 40 {
		t.Fatalf("only %d players get repeat-free games — want at least a full class of 40", players)
	}
	seen := map[string]bool{}
	for i := 0; i < players; i++ {
		p, _, err := g.Register("n", "f", fmt.Sprintf("912%07d", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := g.Start(p.Token); err != nil {
			t.Fatal(err)
		}
		var row model.GamePlayer
		g.db.Where("id = ?", p.ID).First(&row)
		for _, it := range loadItems(&row) {
			if seen[it.QID] {
				t.Fatalf("player %d got repeated question %s before the deck ran out", i, it.QID)
			}
			seen[it.QID] = true
		}
	}
}

// 40 players register and play full games at the same moment.
func TestGameFortyConcurrentPlayers(t *testing.T) {
	g, _ := gameSvc(t)
	g.now = time.Now
	const players = 40
	var wg sync.WaitGroup
	errs := make(chan error, players)
	for i := 0; i < players; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, _, err := g.Register(fmt.Sprintf("p%d", i), "x", fmt.Sprintf("935%07d", i))
			if err != nil {
				errs <- err
				return
			}
			st, err := g.Start(p.Token)
			for q := 0; err == nil && q < st.Total; q++ {
				if st, err = g.Answer(p.Token, q, q%4); err == nil && q < st.Total-1 {
					st, err = g.Next(p.Token)
				}
			}
			if err == nil && st.Phase != "finished" {
				err = fmt.Errorf("player %d ended in phase %s", i, st.Phase)
			}
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	b, _ := g.Board()
	if b.Stats.Finished != players {
		t.Fatalf("finished = %d, want %d", b.Stats.Finished, players)
	}
}

// Every built-in question is valid (4 distinct, non-empty options, a correct
// index) and no question text appears twice.
func TestBuiltInBankIsClean(t *testing.T) {
	g, _ := gameSvc(t)
	defs := model.DefaultGameQuestions()
	if got := g.QuestionCount(); got != int64(len(defs)) {
		t.Fatalf("bank has %d questions, want %d — a duplicate or invalid built-in question was skipped", got, len(defs))
	}
	for _, retired := range model.RetiredBankTexts {
		for _, q := range defs {
			if q.Text == retired {
				t.Fatalf("retired question is still in the built-in bank: %s", retired)
			}
		}
	}
}

// The admin can set any count (e.g. 3, 5, 10): new games get exactly that many
// distinct questions, while a game already in progress keeps its own count.
func TestGameQuestionCountSetting(t *testing.T) {
	g, _ := gameSvc(t)
	early, _, _ := g.Register("a", "b", "9121111111")
	if st, _ := g.Start(early.Token); st.Total != 3 {
		t.Fatalf("default game has %d questions, want 3", st.Total)
	}
	for _, n := range []int{5, 10} {
		gs := g.Settings()
		gs.QuestionsPerGame = n
		if _, err := g.SaveSettings(gs); err != nil {
			t.Fatal(err)
		}
		p, _, _ := g.Register("n", "f", fmt.Sprintf("91222222%02d", n))
		st, err := g.Start(p.Token)
		if err != nil || st.Total != n || len(st.Progress) != n {
			t.Fatalf("n=%d: start gave total=%d err=%v", n, st.Total, err)
		}
		seen := map[string]bool{}
		for q := 0; q < n; q++ {
			if seen[st.Question.Text] {
				t.Fatalf("n=%d: question repeated inside one game", n)
			}
			seen[st.Question.Text] = true
			st, _ = g.Answer(p.Token, q, 0)
			if q < n-1 {
				st, _ = g.Next(p.Token)
			}
		}
		if st.Phase != "finished" || st.Result.Total != n {
			t.Fatalf("n=%d: ended %s with total %d", n, st.Phase, st.Result.Total)
		}
	}
	// the player who started before the change still has 3
	if st, _ := g.State(early.Token); st.Total != 3 {
		t.Fatalf("in-progress game changed to %d questions", st.Total)
	}
}

// Fairness: a correct answer always outweighs any amount of speed. 4 correct
// in 60 s must beat 3 correct in 5 s — in the score and in the ranking.
func TestMoreCorrectAlwaysBeatsFaster(t *testing.T) {
	const limit = int64(20_000) // 20 s per question
	slow4 := ComputeGameScore(4, 5, 60_000, limit)
	fast3 := ComputeGameScore(3, 5, 5_000, limit)
	if slow4 <= fast3 {
		t.Fatalf("score: 4 correct/60s = %d must beat 3 correct/5s = %d", slow4, fast3)
	}
	// even the slowest possible 4 vs the fastest possible 3
	if ComputeGameScore(4, 5, 5*limit, limit) <= ComputeGameScore(3, 5, 0, limit) {
		t.Fatal("score: the slowest 4-correct run must still beat the fastest 3-correct run")
	}
	rows := []BoardRow{
		{ID: "fast3", Correct: 3, TotalMs: 5_000, FinishedAt: 1},
		{ID: "slow4", Correct: 4, TotalMs: 60_000, FinishedAt: 2},
		{ID: "tie-slower", Correct: 3, TotalMs: 9_000, FinishedAt: 0},
	}
	RankFinished(rows)
	if rows[0].ID != "slow4" || rows[1].ID != "fast3" || rows[2].ID != "tie-slower" {
		t.Fatalf("ranking = %s, %s, %s; want slow4, fast3, tie-slower", rows[0].ID, rows[1].ID, rows[2].ID)
	}
}

// The easy/hard order the admin asked for: 5 = E E E H H, 10 = 3E 2H 4E 1H.
func TestDifficultyPlan(t *testing.T) {
	want := map[int]string{
		3:  "EEH",
		5:  "EEEHH",
		10: "EEEHHEEEEH",
		15: "EEEHHEEEEHEEEHH",
		20: "EEEHHEEEEHEEEHHEEEEH",
	}
	for n, w := range want {
		got := ""
		for _, lv := range model.DifficultyPlan(n) {
			got += map[string]string{model.DifficultyEasy: "E", model.DifficultyHard: "H"}[lv]
		}
		if got != w {
			t.Errorf("plan(%d) = %s, want %s", n, got, w)
		}
	}
	for n := 1; n <= 50; n++ {
		if l := len(model.DifficultyPlan(n)); l != n {
			t.Fatalf("plan(%d) has %d entries", n, l)
		}
	}
}

// Games are dealt in plan order from the matching difficulty.
func TestGameDealsByDifficulty(t *testing.T) {
	g, _ := gameSvc(t)
	for _, n := range []int{5, 10} {
		gs := g.Settings()
		gs.QuestionsPerGame = n
		g.SaveSettings(gs)
		for k := 0; k < 4; k++ {
			p, _, _ := g.Register("n", "f", fmt.Sprintf("9133%03d%03d", n, k))
			if _, err := g.Start(p.Token); err != nil {
				t.Fatal(err)
			}
			var row model.GamePlayer
			g.db.Where("id = ?", p.ID).First(&row)
			plan := model.DifficultyPlan(n)
			for i, it := range loadItems(&row) {
				if it.Difficulty != plan[i] {
					t.Fatalf("n=%d game %d question %d is %s, want %s", n, k, i+1, it.Difficulty, plan[i])
				}
			}
		}
	}
}

// With no hard questions in the bank a game still gets its full count.
func TestDealFallsBackWhenALevelIsEmpty(t *testing.T) {
	g, _ := gameSvc(t)
	g.db.Model(&model.GameQuestion{}).Where("1 = 1").Update("difficulty", model.DifficultyEasy)
	g.bankRev.Add(1)
	gs := g.Settings()
	gs.QuestionsPerGame = 5
	g.SaveSettings(gs)
	p, _, _ := g.Register("n", "f", "9134000000")
	st, err := g.Start(p.Token)
	if err != nil || st.Total != 5 {
		t.Fatalf("got %d questions, err %v; want 5 easy ones as fallback", st.Total, err)
	}
}

// Changing the round mid-competition is flagged so the admin knows the
// comparison is not like-for-like.
func TestBoardFlagsMixedRounds(t *testing.T) {
	g, _ := gameSvc(t)
	play := func(phone string) {
		p, _, _ := g.Register("n", "f", phone)
		st, _ := g.Start(p.Token)
		for q := 0; q < st.Total; q++ {
			g.Answer(p.Token, q, 0)
			if q < st.Total-1 {
				g.Next(p.Token)
			}
		}
	}
	play("9135000001")
	if b, _ := g.Board(); b.MixedRounds {
		t.Fatal("one round only — must not be flagged")
	}
	gs := g.Settings()
	gs.QuestionsPerGame = 5
	g.SaveSettings(gs)
	play("9135000002")
	if b, _ := g.Board(); !b.MixedRounds {
		t.Fatal("3- and 5-question games finished — must be flagged")
	}
}
