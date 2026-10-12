package model

import "testing"

func TestCleanFaText(t *testing.T) {
	cases := map[string]string{
		"کدام کلمه‌ی کلیدی در جاوا برای ساختن یک شیء جدید استفاده می‌شود؟": "کدام کلمه کلیدی در جاوا برای ساختن یک شیء جدید استفاده می‌شود؟",
		"معمولاً":     "معمولا",
		"(مثلاً int)": "(مثلا int)",
		"اندازه‌ی آرایه‌ی ورودی":      "اندازه آرایه ورودی",
		"خانهٔ اول":                   "خانه اول",
		"کلمه‌ی":                      "کلمه",
		"یک برنامه‌ای ساده":           "یک برنامه‌ای ساده", // indefinite "ای" stays
		"داده‌ها و کتابخانه‌های جاوا": "داده‌ها و کتابخانه‌های جاوا",
		"شیء":                         "شیء",
		"System.out.println(\"hi\");": "System.out.println(\"hi\");",
	}
	for in, want := range cases {
		if got := CleanFaText(in); got != want {
			t.Errorf("CleanFaText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuiltInBankFollowsFaStyle(t *testing.T) {
	for _, q := range DefaultGameQuestions() {
		if CleanFaQuestion(&q) {
			t.Errorf("built-in question not in house style: %q", q.Text)
		}
	}
}
