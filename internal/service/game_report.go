package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/dotinschool/dotinschool/internal/model"
	"github.com/dotinschool/dotinschool/internal/xlsx"
)

// The full end-of-competition report, as an Excel workbook (right-to-left):
//
//  1. خلاصه       — totals, the round's settings, the podium, accuracy per category/level
//  2. رتبه‌بندی    — one row per player: rank, contact, correct/wrong/timeout, time, score,
//     correct answers per category
//  3. پاسخ‌ها      — one row per answered question of every player
//  4. تحلیل سوالات — one row per question that was dealt: how often, how many got it right
func (s *GameService) ReportXLSX() ([]byte, error) {
	var players []model.GamePlayer
	if err := s.db.Omit("token").Order("created_at ASC").Find(&players).Error; err != nil {
		return nil, err
	}
	now := s.now()
	gs := s.Settings()

	// rank finished players exactly like the board
	var finished []BoardRow
	byID := map[string]*model.GamePlayer{}
	for i := range players {
		p := &players[i]
		byID[p.ID] = p
		if p.Status == model.GameFinished {
			finished = append(finished, BoardRow{ID: p.ID, Correct: p.Correct, TotalMs: p.TotalMs, FinishedAt: p.FinishedAt})
		}
	}
	RankFinished(finished)
	rank := map[string]int{}
	for _, r := range finished {
		rank[r.ID] = r.Rank
	}
	// report order: finished by rank, then playing, then registered-only
	ordered := make([]*model.GamePlayer, 0, len(players))
	for _, r := range finished {
		ordered = append(ordered, byID[r.ID])
	}
	for _, st := range []string{model.GamePlaying, model.GameRegistered} {
		for i := range players {
			if players[i].Status == st {
				ordered = append(ordered, &players[i])
			}
		}
	}

	// the categories that appear in any dealt game, mix order first
	var cats []string
	seenCat := map[string]bool{}
	addCat := func(c string) {
		if !seenCat[c] {
			seenCat[c] = true
			cats = append(cats, c)
		}
	}
	if gs.MixOn {
		for _, m := range gs.Mix {
			addCat(m.Category)
		}
	}
	items := map[string][]model.GameItem{}
	for _, p := range ordered {
		items[p.ID] = loadItems(p)
		for _, it := range items[p.ID] {
			addCat(it.Category)
		}
	}

	// ---- accumulate stats over finished games ----
	type agg struct {
		shown, correct, timeout int
		ms                      int64
	}
	type qkey struct{ text, cat, lv string }
	perCatLv := map[[2]string]*agg{}
	perQ := map[qkey]*agg{}
	var qOrder []qkey
	for _, r := range finished {
		for _, it := range items[r.ID] {
			if !it.Answered {
				continue
			}
			ck := [2]string{it.Category, it.Difficulty}
			if perCatLv[ck] == nil {
				perCatLv[ck] = &agg{}
			}
			qk := qkey{it.Text, it.Category, it.Difficulty}
			if perQ[qk] == nil {
				perQ[qk] = &agg{}
				qOrder = append(qOrder, qk)
			}
			for _, a := range []*agg{perCatLv[ck], perQ[qk]} {
				a.shown++
				a.ms += it.Ms
				if it.IsCorrect {
					a.correct++
				}
				if it.Choice == -1 {
					a.timeout++
				}
			}
		}
	}

	// ---- 1. summary ----
	var sumScore, sumCorrect int
	playing, waiting := 0, 0
	for _, p := range players {
		switch p.Status {
		case model.GameFinished:
			sumScore += p.Score
			sumCorrect += p.Correct
		case model.GamePlaying:
			playing++
		default:
			waiting++
		}
	}
	avg := func(sum, n int) any {
		if n == 0 {
			return "—"
		}
		return round1(float64(sum) / float64(n))
	}
	hdr := func(cols ...string) []any {
		row := make([]any, len(cols))
		for i, c := range cols {
			row[i] = xlsx.Cell{V: c, S: xlsx.Header}
		}
		return row
	}
	label := func(s string) xlsx.Cell { return xlsx.Cell{V: s, S: xlsx.Bold} }

	sum := [][]any{
		{xlsx.Cell{V: "گزارش کامل Game مدرسه داتین", S: xlsx.Title}},
		{label("زمان تهیه‌ی گزارش"), jalaliDateTime(now)},
		{},
		{label("تعداد ثبت‌نام"), len(players)},
		{label("بازی تمام‌شده"), len(finished)},
		{label("در حال بازی"), playing},
		{label("ثبت‌نام بدون شروع بازی"), waiting},
		{label("میانگین جواب درست"), avg(sumCorrect, len(finished))},
		{label("میانگین امتیاز"), avg(sumScore, len(finished))},
		{},
		{label("تعداد سوال هر بازی"), gs.QuestionsPerGame},
		{label("زمان هر سوال (ثانیه)"), gs.SecondsPerQuestion},
	}
	if gs.MixOn {
		for _, m := range gs.Mix {
			sum = append(sum, []any{label("ترکیب: " + m.Category), fmt.Sprintf("%d آسان + %d سخت", m.Easy, m.Hard)})
		}
	}
	sum = append(sum, []any{}, []any{xlsx.Cell{V: "نفرات برتر", S: xlsx.Title}},
		hdr("رتبه", "نام و نام خانوادگی", "موبایل", "جواب درست", "زمان کل (ثانیه)", "امتیاز"))
	medals := []string{"🥇 اول", "🥈 دوم", "🥉 سوم"}
	for i, r := range finished {
		p := byID[r.ID]
		if i == 3 || p.Correct == 0 {
			break
		}
		g := func(v any) xlsx.Cell { return xlsx.Cell{V: v, S: xlsx.Gold} }
		sum = append(sum, []any{g(medals[i]), g(p.FirstName + " " + p.LastName), g("0" + p.Phone),
			g(fmt.Sprintf("%d از %d", p.Correct, p.Total)), g(secs(p.TotalMs)), g(p.Score)})
	}
	sum = append(sum, []any{}, []any{xlsx.Cell{V: "درصد پاسخ درست به تفکیک دسته و سطح", S: xlsx.Title}},
		hdr("دسته", "سطح", "تعداد پاسخ", "درست", "زمان تمام شد", "درصد درست", "میانگین زمان (ثانیه)"))
	for _, c := range cats {
		for _, lv := range []string{model.DifficultyEasy, model.DifficultyHard} {
			a := perCatLv[[2]string{c, lv}]
			if a == nil {
				continue
			}
			sum = append(sum, []any{c, levelFa(lv), a.shown, a.correct, a.timeout, pct(a.correct, a.shown), secs(a.ms / int64(a.shown))})
		}
	}

	// ---- 2. ranking ----
	rankHead := []string{"رتبه", "نام", "نام خانوادگی", "موبایل", "وضعیت", "جواب درست", "جواب غلط", "زمان تمام شد", "تعداد سوال"}
	for _, c := range cats {
		rankHead = append(rankHead, "درست در "+c)
	}
	rankHead = append(rankHead, "زمان کل (ثانیه)", "میانگین هر سوال (ثانیه)", "امتیاز", "شروع بازی", "پایان بازی", "ثبت‌نام")
	rankRows := [][]any{hdr(rankHead...)}
	for _, p := range ordered {
		wrong, timeout, answered := 0, 0, 0
		perCat := map[string][2]int{} // correct, dealt
		for _, it := range items[p.ID] {
			pc := perCat[it.Category]
			pc[1]++
			if it.Answered {
				answered++
				switch {
				case it.IsCorrect:
					pc[0]++
				case it.Choice == -1:
					timeout++
				default:
					wrong++
				}
			}
			perCat[it.Category] = pc
		}
		var rk any = "—"
		if r := rank[p.ID]; r > 0 {
			rk = r
		}
		row := []any{rk, p.FirstName, p.LastName, "0" + p.Phone, statusFa(p.Status), p.Correct, wrong, timeout, p.Total}
		for _, c := range cats {
			if pc, ok := perCat[c]; ok {
				row = append(row, fmt.Sprintf("%d از %d", pc[0], pc[1]))
			} else {
				row = append(row, "—")
			}
		}
		var avgQ any = "—"
		if answered > 0 {
			avgQ = secs(p.TotalMs / int64(answered))
		}
		row = append(row, secs(p.TotalMs), avgQ, p.Score, msTime(p.StartedAt), msTime(p.FinishedAt), msTime(p.CreatedAt))
		if r := rank[p.ID]; r >= 1 && r <= 3 && p.Correct > 0 {
			for i := range row {
				row[i] = xlsx.Cell{V: row[i], S: xlsx.Gold}
			}
		}
		rankRows = append(rankRows, row)
	}
	rankWidths := []float64{7, 14, 18, 14, 13, 10, 10, 11, 10}
	for range cats {
		rankWidths = append(rankWidths, 15)
	}
	rankWidths = append(rankWidths, 13, 14, 9, 20, 20, 20)

	// ---- 3. every answer ----
	ansRows := [][]any{hdr("رتبه", "نام و نام خانوادگی", "موبایل", "سوال", "دسته", "سطح", "متن سوال",
		"جواب بازیکن", "جواب درست", "نتیجه", "زمان پاسخ (ثانیه)")}
	for _, p := range ordered {
		var rk any = "—"
		if r := rank[p.ID]; r > 0 {
			rk = r
		}
		for i, it := range items[p.ID] {
			if !it.Answered {
				continue
			}
			res, st, choice := "غلط", xlsx.Bad, "—"
			switch {
			case it.IsCorrect:
				res, st = "درست", xlsx.Good
			case it.Choice == -1:
				res, st = "زمان تمام شد", xlsx.Muted
			}
			if it.Choice >= 0 && it.Choice < 4 {
				choice = it.Options[it.Choice]
			}
			correct := ""
			if it.Correct >= 0 && it.Correct < 4 {
				correct = it.Options[it.Correct]
			}
			ansRows = append(ansRows, []any{rk, p.FirstName + " " + p.LastName, "0" + p.Phone, i + 1, it.Category,
				levelFa(it.Difficulty), it.Text, choice, correct, xlsx.Cell{V: res, S: st}, secs(it.Ms)})
		}
	}

	// ---- 4. per question, hardest first ----
	sort.SliceStable(qOrder, func(i, j int) bool {
		a, b := perQ[qOrder[i]], perQ[qOrder[j]]
		return a.correct*b.shown < b.correct*a.shown // lower correct-rate first
	})
	qRows := [][]any{hdr("متن سوال", "دسته", "سطح", "دفعات نمایش", "درست", "غلط", "زمان تمام شد", "درصد درست", "میانگین زمان (ثانیه)")}
	for _, k := range qOrder {
		a := perQ[k]
		qRows = append(qRows, []any{k.text, k.cat, levelFa(k.lv), a.shown, a.correct, a.shown - a.correct - a.timeout,
			a.timeout, pct(a.correct, a.shown), secs(a.ms / int64(a.shown))})
	}

	return xlsx.Bytes([]xlsx.Sheet{
		{Name: "خلاصه", RTL: true, Rows: sum, Widths: []float64{30, 30, 16, 14, 16, 14, 18}},
		{Name: "رتبه‌بندی", RTL: true, Rows: rankRows, Widths: rankWidths, HeaderRow: 1},
		{Name: "پاسخ‌ها", RTL: true, Rows: ansRows, Widths: []float64{7, 22, 14, 7, 14, 8, 55, 26, 26, 13, 13}, HeaderRow: 1},
		{Name: "تحلیل سوالات", RTL: true, Rows: qRows, Widths: []float64{60, 14, 8, 12, 9, 9, 12, 11, 16}, HeaderRow: 1},
	})
}

func round1(f float64) float64 { return float64(int64(f*10+0.5)) / 10 }

// secs is milliseconds as seconds with one decimal.
func secs(ms int64) float64 { return round1(float64(ms) / 1000) }

func pct(n, of int) any {
	if of == 0 {
		return "—"
	}
	return fmt.Sprintf("%d٪", (n*100+of/2)/of)
}

func levelFa(lv string) string {
	if lv == model.DifficultyHard {
		return "سخت"
	}
	return "آسان"
}

func statusFa(st string) string {
	switch st {
	case model.GameFinished:
		return "پایان بازی"
	case model.GamePlaying:
		return "در حال بازی"
	}
	return "ثبت‌نام"
}

func msTime(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	return jalaliDateTime(time.UnixMilli(ms))
}

// jalaliDateTime formats t (server-local time, Asia/Tehran in Docker) as
// "1405/07/14 15:04".
func jalaliDateTime(t time.Time) string {
	t = t.Local()
	y, m, d := gregorianToJalali(t.Year(), int(t.Month()), t.Day())
	return fmt.Sprintf("%04d/%02d/%02d %02d:%02d", y, m, d, t.Hour(), t.Minute())
}

// gregorianToJalali is the standard arithmetic conversion (valid for any
// date this app will ever see).
func gregorianToJalali(gy, gm, gd int) (int, int, int) {
	gdm := [12]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	gy2 := gy
	if gm > 2 {
		gy2 = gy + 1
	}
	days := 355666 + 365*gy + (gy2+3)/4 - (gy2+99)/100 + (gy2+399)/400 + gd + gdm[gm-1]
	jy := -1595 + 33*(days/12053)
	days %= 12053
	jy += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		jy += (days - 1) / 365
		days = (days - 1) % 365
	}
	var jm, jd int
	if days < 186 {
		jm, jd = 1+days/31, 1+days%31
	} else {
		jm, jd = 7+(days-186)/30, 1+(days-186)%30
	}
	return jy, jm, jd
}
