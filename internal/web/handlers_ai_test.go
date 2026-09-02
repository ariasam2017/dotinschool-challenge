package web

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// The first line of the AI review must be split off as AI_GENERATED_PERCENT.
func TestAIPercentParsing(t *testing.T) {
	cases := []struct {
		in    string
		score int
		body  string
	}{
		{"AI_GENERATED_PERCENT: 73\n## خلاصه\nمتن", 73, "## خلاصه\nمتن"},
		{"  ai generated percent = 100 %\nrest", 100, "rest"},
		{"AI_GENERATED_PERCENT: 250\nx", 100, "x"}, // clamped
		{"no header here\njust text", -1, "no header here\njust text"},
	}
	for _, c := range cases {
		review := c.in
		var score *int
		if m := aiPercentRe.FindStringSubmatchIndex(review); m != nil {
			n, _ := strconv.Atoi(review[m[2]:m[3]])
			if n < 0 {
				n = 0
			}
			if n > 100 {
				n = 100
			}
			score = &n
			review = strings.TrimLeft(review[m[1]:], " \t\r\n")
		}
		if c.score < 0 {
			if score != nil {
				t.Errorf("%q: expected no score, got %d", c.in, *score)
			}
		} else if score == nil || *score != c.score {
			t.Errorf("%q: score = %v, want %d", c.in, score, c.score)
		}
		if review != c.body {
			t.Errorf("%q: body = %q, want %q", c.in, review, c.body)
		}
	}
}

// number | null | absent must all remain distinguishable when patching.
func TestApplyNullableInt(t *testing.T) {
	seven := 7
	start := &seven

	// absent -> untouched
	dst := start
	applyNullableInt(nil, &dst, false)
	if dst == nil || *dst != 7 {
		t.Fatal("absent field must leave the value untouched")
	}

	// explicit null -> nil
	dst = start
	applyNullableInt(json.RawMessage("null"), &dst, false)
	if dst != nil {
		t.Fatal("explicit null must set the field to nil")
	}

	// value with clamp
	dst = nil
	applyNullableInt(json.RawMessage("512"), &dst, true)
	if dst == nil || *dst != 100 {
		t.Fatalf("clamped value = %v, want 100", dst)
	}

	// value without clamp
	dst = nil
	applyNullableInt(json.RawMessage("42"), &dst, false)
	if dst == nil || *dst != 42 {
		t.Fatalf("value = %v, want 42", dst)
	}
}
