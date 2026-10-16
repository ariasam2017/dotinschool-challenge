package service

import (
	"testing"

	"github.com/dotinschool/dotinschool-challenge/internal/config"
)

// A browser password manager autofilling the panel login username into the
// "model" field must not reach the upstream verbatim — it falls back to the
// default model instead (an OpenAI-compatible gateway rejects a model literally
// named "administrator").
func TestModelOrDefault(t *testing.T) {
	a := &AIService{cfg: &config.Config{AdminUsername: "administrator"}}

	cases := []struct {
		in, want string
	}{
		{"gpt-4o-mini", "gpt-4o-mini"},
		{"  llama-3.3-70b-versatile  ", "llama-3.3-70b-versatile"},
		{"", AIDefaultModel},
		{"   ", AIDefaultModel},
		{"administrator", AIDefaultModel},
		{"ADMINISTRATOR", AIDefaultModel},
		{"Administrator", AIDefaultModel},
	}
	for _, c := range cases {
		if got := a.ModelOrDefault(c.in); got != c.want {
			t.Errorf("ModelOrDefault(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// no admin username configured -> only empty falls back
	b := &AIService{cfg: &config.Config{}}
	if got := b.ModelOrDefault("administrator"); got != "administrator" {
		t.Errorf("with no AdminUsername, ModelOrDefault(%q) = %q, want passthrough", "administrator", got)
	}
	if got := b.ModelOrDefault(""); got != AIDefaultModel {
		t.Errorf("ModelOrDefault(%q) = %q, want %q", "", got, AIDefaultModel)
	}
}
