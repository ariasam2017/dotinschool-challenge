package model

import "unicode"

// CleanFaText applies the bank's Persian style rules to one question text or
// option: tanvin is dropped ("معمولاً" -> "معمولا") and the written ezafe "ی"
// after a final "ه" is removed ("کلمه‌ی کلیدی" -> "کلمه کلیدی"). Only a "ه" +
// ZWNJ + "ی" that ends a word is touched, so "ه‌ای"/"ه‌ها"/"ه‌یی" stay intact.
func CleanFaText(s string) string {
	rs := []rune(s)
	out := make([]rune, 0, len(rs))
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r >= 'ً' && r <= 'ٍ' { // fathatan, dammatan, kasratan
			continue
		}
		if r == 'ه' && i+2 < len(rs) && rs[i+1] == '‌' &&
			(rs[i+2] == 'ی' || rs[i+2] == 'ي') && endsWord(rs, i+3) {
			out = append(out, r)
			i += 2
			continue
		}
		if r == 'ه' && i+1 < len(rs) && rs[i+1] == 'ٔ' && endsWord(rs, i+2) { // "هٔ"
			out = append(out, r)
			i++
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// endsWord reports whether position i in rs is past the end of a word.
func endsWord(rs []rune, i int) bool {
	if i >= len(rs) {
		return true
	}
	r := rs[i]
	return !(unicode.IsLetter(r) || unicode.Is(unicode.Mn, r) || r == '‌' || r == '‍')
}

// CleanFaQuestion applies CleanFaText to the text and every option, and
// reports whether anything changed.
func CleanFaQuestion(q *GameQuestion) bool {
	text := CleanFaText(q.Text)
	opts := q.Options()
	changed := text != q.Text
	for i := range opts {
		c := CleanFaText(opts[i])
		if c != opts[i] {
			changed = true
		}
		opts[i] = c
	}
	if !changed {
		return false
	}
	q.Text = text
	q.SetOptions(opts)
	return true
}
