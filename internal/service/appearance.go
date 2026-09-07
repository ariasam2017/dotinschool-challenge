package service

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
)

// ---- Design Studio: the panel-managed appearance layer --------------------
//
// Two settings rows: appearanceDraft (what the admin edits) and
// appearancePublished (what visitors get, folded into GET /api/config).
// Everything is validated hard on the way in — the value ends up inside a
// <style> tag on every visitor's page.

const (
	appearanceMaxTokenVal = 200
	appearanceMaxText     = 400
	appearanceMaxCSS      = 20000
	appearanceMaxImage    = 512 * 1024 // 512 KB data: URL
	appearanceMaxTexts    = 200
	appearanceMaxFind     = 400
	appearanceMaxReplace  = 2000
	appearanceMaxSelector = 400
)

// a CSS selector may only contain these characters — enough for tag/class/id/
// child+descendant combinators and :nth-of-type(), nothing that could break
// out of a rule ({ } ; @ < > backslash * are rejected, > is allowed).
var appearanceSelectorRe = regexp.MustCompile(`^[A-Za-z0-9 .#:>()\[\]="'\-_,\s]+$`)

// allowedAppearanceTokens is the whitelist of CSS custom properties the admin
// may set. Anything else in the tokens/dark maps is dropped.
var allowedAppearanceTokens = map[string]bool{
	"primary": true, "primary-hover": true, "primary-soft": true,
	"accent": true, "accent-soft": true,
	"bg": true, "surface": true, "surface-2": true, "surface-3": true,
	"text": true, "text-dim": true, "text-faint": true,
	"border": true, "border-strong": true,
	"danger": true, "warning": true, "success": true, "info": true,
	"on-primary": true, "on-accent": true, "ring": true,
	"radius": true, "radius-sm": true, "radius-lg": true, "radius-pill": true,
	"font-ui":  true,
	"shadow-1": true, "shadow-2": true, "shadow-3": true,
	"space-3": true, "space-4": true, "space-5": true, "space-6": true,
	"header-h": true, "sidebar-w": true,
	"code-bg": true, "code-text": true,
}

// a CSS value may only contain these characters (no ; { } @ : < > backslash).
var appearanceValueRe = regexp.MustCompile(`^[A-Za-z0-9#%.,()\-_'"/ ]+$`)

// url(...) is only allowed when it points at an inline data: URI.
var appearanceBadURLRe = regexp.MustCompile(`(?i)url\(\s*['"]?\s*(?:https?:|//|/)`)

// raw CSS that is never allowed through.
var appearanceCSSBlockRe = regexp.MustCompile(
	`(?i)(@import|</\s*style|<\s*script|javascript:|expression\s*\(|-moz-binding|url\(\s*['"]?\s*(?:https?:|//))`)

var appearanceImageRe = regexp.MustCompile(`^data:image/(png|jpe?g|webp|gif|svg\+xml);base64,[A-Za-z0-9+/=\s]+$`)

var appearanceAligns = map[string]bool{"": true, "start": true, "center": true, "end": true}

var overrideFontSizeRe = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,2})?(px|rem|em|%)$`)
var overrideFontWeights = map[string]bool{
	"": true, "300": true, "400": true, "500": true, "600": true,
	"700": true, "800": true, "900": true, "normal": true, "bold": true,
}
var overrideTextAligns = map[string]bool{
	"": true, "right": true, "left": true, "center": true, "justify": true,
}

// AppearanceDraftRaw / AppearancePublishedRaw return the stored JSON, or "{}".
func (s *SettingService) AppearanceDraftRaw() json.RawMessage {
	return s.appearanceRaw(model.KeyAppearanceDraft)
}

func (s *SettingService) AppearancePublishedRaw() json.RawMessage {
	return s.appearanceRaw(model.KeyAppearancePublished)
}

func (s *SettingService) appearanceRaw(key string) json.RawMessage {
	if raw, ok := s.getRaw(key); ok && len(raw) > 0 {
		return raw
	}
	return json.RawMessage("{}")
}

// SaveAppearanceDraft validates and stores the draft.
func (s *SettingService) SaveAppearanceDraft(a *model.Appearance) error {
	if err := sanitizeAppearance(a); err != nil {
		return err
	}
	a.UpdatedAt = time.Now().UnixMilli()
	return s.Set(model.KeyAppearanceDraft, a)
}

// PublishAppearance copies the current draft over the published row.
func (s *SettingService) PublishAppearance() error {
	return s.SetRaw(model.KeyAppearancePublished, s.AppearanceDraftRaw())
}

// RevertAppearanceDraft resets the draft back to the published row.
func (s *SettingService) RevertAppearanceDraft() error {
	return s.SetRaw(model.KeyAppearanceDraft, s.AppearancePublishedRaw())
}

// ResetAppearance clears both rows — back to the built-in design.
func (s *SettingService) ResetAppearance() error {
	empty := json.RawMessage("{}")
	if err := s.SetRaw(model.KeyAppearanceDraft, empty); err != nil {
		return err
	}
	return s.SetRaw(model.KeyAppearancePublished, empty)
}

// clipRunes trims a string to at most n runes (package `web` has its own clip).
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func sanitizeAppearance(a *model.Appearance) error {
	a.Tokens = cleanTokenMap(a.Tokens)
	a.Dark = cleanTokenMap(a.Dark)

	h := &a.Sections.Hero
	h.Title = clipRunes(strings.TrimSpace(h.Title), appearanceMaxText)
	h.Subtitle = clipRunes(strings.TrimSpace(h.Subtitle), appearanceMaxText)
	if !appearanceAligns[h.Align] {
		h.Align = ""
	}
	if err := checkCSSValue("hero.background", h.Background); err != nil {
		return err
	}
	if err := checkCSSValue("hero.minHeight", h.MinHeight); err != nil {
		return err
	}
	if h.Image != "" {
		if len(h.Image) > appearanceMaxImage {
			return apperr.InvalidInput("تصویر پس‌زمینه‌ی هیرو بیش از حد بزرگ است (حداکثر ۵۱۲ کیلوبایت).")
		}
		if !appearanceImageRe.MatchString(strings.TrimSpace(h.Image)) {
			return apperr.InvalidInput("فرمت تصویر پس‌زمینه‌ی هیرو نامعتبر است.")
		}
	}

	hd := &a.Sections.Header
	hd.Brand = clipRunes(strings.TrimSpace(hd.Brand), 80)
	if err := checkCSSValue("header.background", hd.Background); err != nil {
		return err
	}

	a.CustomCSS = strings.TrimSpace(a.CustomCSS)
	if len(a.CustomCSS) > appearanceMaxCSS {
		return apperr.InvalidInput("حجم CSS سفارشی بیش از حد مجاز است (حداکثر ۲۰ هزار نویسه).")
	}
	if a.CustomCSS != "" && appearanceCSSBlockRe.MatchString(a.CustomCSS) {
		return apperr.InvalidInput("CSS سفارشی شامل موردی است که مجاز نیست (@import، url(http…)، script، javascript:، expression).")
	}

	a.Texts = cleanTextReplacements(a.Texts)
	a.Overrides = cleanOverrides(a.Overrides)
	return nil
}

func cleanOverrides(in []model.ElementOverride) []model.ElementOverride {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.ElementOverride, 0, len(in))
	for _, o := range in {
		sel := strings.TrimSpace(o.Selector)
		if sel == "" || len(sel) > appearanceMaxSelector || !appearanceSelectorRe.MatchString(sel) {
			continue
		}
		o.Selector = sel
		o.Text = clipRunes(o.Text, appearanceMaxReplace)
		o.Label = clipRunes(strings.TrimSpace(o.Label), 120)

		// Fold legacy flat style fields into the All layer, then drop them.
		if bagEmpty(o.All) {
			o.All = model.StyleBag{
				FontSize: o.FontSize, FontWeight: o.FontWeight, TextAlign: o.TextAlign,
				Color: o.Color, Background: o.Background, Padding: o.Padding, Margin: o.Margin,
				BorderWidth: o.BorderWidth, BorderColor: o.BorderColor,
				BorderRadius: o.BorderRadius, Width: o.Width, MaxWidth: o.MaxWidth,
			}
		}
		o.FontSize, o.FontWeight, o.TextAlign = "", "", ""
		o.Color, o.Background, o.Padding, o.Margin = "", "", "", ""
		o.BorderWidth, o.BorderColor, o.BorderRadius = "", "", ""
		o.Width, o.MaxWidth = "", ""

		o.All = cleanStyleBag(o.All)
		o.Mobile = cleanStyleBag(o.Mobile)
		o.Desktop = cleanStyleBag(o.Desktop)

		anyHidden := o.Hidden || o.HiddenMobile || o.HiddenDesktop
		if !o.SetText && !anyHidden && o.Text == "" &&
			bagEmpty(o.All) && bagEmpty(o.Mobile) && bagEmpty(o.Desktop) {
			continue // nothing to do
		}
		out = append(out, o)
		if len(out) >= appearanceMaxTexts {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanTextReplacements(in []model.TextReplacement) []model.TextReplacement {
	if len(in) == 0 {
		return nil
	}
	out := make([]model.TextReplacement, 0, len(in))
	for _, t := range in {
		find := clipRunes(strings.TrimSpace(t.Find), appearanceMaxFind)
		if find == "" {
			continue
		}
		mode := t.Mode
		if mode != "contains" {
			mode = "exact"
		}
		out = append(out, model.TextReplacement{
			Find:    find,
			Replace: clipRunes(t.Replace, appearanceMaxReplace),
			Mode:    mode,
		})
		if len(out) >= appearanceMaxTexts {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanTokenMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		k = strings.TrimSpace(strings.TrimPrefix(k, "--"))
		v = strings.TrimSpace(v)
		if v == "" || !allowedAppearanceTokens[k] {
			continue
		}
		if len(v) > appearanceMaxTokenVal || !appearanceValueRe.MatchString(v) || appearanceBadURLRe.MatchString(v) {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// safeCSSVal returns a trimmed CSS value, or "" if it fails the charset /
// length / no-external-url checks. Used for the per-element style overrides,
// where an invalid value is simply dropped rather than failing the save.
func safeCSSVal(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > appearanceMaxTokenVal ||
		!appearanceValueRe.MatchString(v) || appearanceBadURLRe.MatchString(v) {
		return ""
	}
	return v
}

func bagEmpty(b model.StyleBag) bool {
	return b == (model.StyleBag{})
}

// cleanStyleBag validates each declaration, dropping anything invalid.
func cleanStyleBag(b model.StyleBag) model.StyleBag {
	b.FontSize = strings.TrimSpace(b.FontSize)
	if b.FontSize != "" && !overrideFontSizeRe.MatchString(b.FontSize) {
		b.FontSize = ""
	}
	if !overrideFontWeights[b.FontWeight] {
		b.FontWeight = ""
	}
	if !overrideTextAligns[b.TextAlign] {
		b.TextAlign = ""
	}
	b.Color = safeCSSVal(b.Color)
	b.Background = safeCSSVal(b.Background)
	b.Padding = safeCSSVal(b.Padding)
	b.Margin = safeCSSVal(b.Margin)
	b.BorderWidth = safeCSSVal(b.BorderWidth)
	b.BorderColor = safeCSSVal(b.BorderColor)
	b.BorderRadius = safeCSSVal(b.BorderRadius)
	b.Width = safeCSSVal(b.Width)
	b.MaxWidth = safeCSSVal(b.MaxWidth)
	return b
}

func checkCSSValue(field, v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if len(v) > appearanceMaxTokenVal || !appearanceValueRe.MatchString(v) || appearanceBadURLRe.MatchString(v) {
		return apperr.InvalidInput("مقدار «" + field + "» نامعتبر است.")
	}
	return nil
}
