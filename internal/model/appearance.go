package model

// Setting keys for the panel-managed visual customisation layer.
const (
	KeyAppearanceDraft     = "appearanceDraft"
	KeyAppearancePublished = "appearancePublished"
)

// Appearance is the admin-editable visual layer applied to the user panel at
// runtime as CSS custom properties + a stylesheet — no rebuild, no source
// edit. Every field is optional; anything empty falls back to the built-in
// design. `Draft` is what the admin is editing; `Published` is what visitors
// see. The user panel picks up a publish within ~1s (it already polls
// GET /api/config).
type Appearance struct {
	// Tokens maps a CSS custom-property name (no leading "--") to a value,
	// e.g. {"primary": "#7c3aed", "radius": "14px"}. Applied to every theme.
	Tokens map[string]string `json:"tokens,omitempty"`
	// Dark is the same, but only for dark mode.
	Dark map[string]string `json:"dark,omitempty"`
	// Sections holds a few structured per-area overrides.
	Sections AppearanceSections `json:"sections"`
	// CustomCSS is raw CSS injected into the user panel (sanitised).
	CustomCSS string `json:"customCss,omitempty"`
	// Texts are find→replace rules applied to visible text on the user panel.
	Texts []TextReplacement `json:"texts,omitempty"`
	// Overrides target one element (by CSS selector, chosen with the picker)
	// to change its text and/or hide it.
	Overrides []ElementOverride `json:"overrides,omitempty"`
	// UpdatedAt is set server-side on every draft save (epoch ms).
	UpdatedAt int64 `json:"updatedAt,omitempty"`
}

// StyleBag is a set of CSS declarations for one element. Each non-empty field
// becomes `prop: value !important`.
type StyleBag struct {
	FontSize     string `json:"fontSize,omitempty"`
	FontWeight   string `json:"fontWeight,omitempty"`
	TextAlign    string `json:"textAlign,omitempty"`
	Color        string `json:"color,omitempty"`
	Background   string `json:"background,omitempty"`
	Padding      string `json:"padding,omitempty"`
	Margin       string `json:"margin,omitempty"`
	BorderWidth  string `json:"borderWidth,omitempty"`
	BorderColor  string `json:"borderColor,omitempty"`
	BorderRadius string `json:"borderRadius,omitempty"`
	Width        string `json:"width,omitempty"`
	MaxWidth     string `json:"maxWidth,omitempty"`
}

// ElementOverride is one picked element and what to do with it. Styles live in
// three layers: All (every viewport), plus Mobile and Desktop which override
// All inside their media query. Hidden flags are scoped the same way.
type ElementOverride struct {
	Selector string `json:"selector"`
	Label    string `json:"label,omitempty"`
	SetText  bool   `json:"setText,omitempty"`
	// Text is the every-viewport copy; TextMobile / TextDesktop override it
	// inside their media query (empty = inherit Text).
	Text        string `json:"text,omitempty"`
	TextMobile  string `json:"textMobile,omitempty"`
	TextDesktop string `json:"textDesktop,omitempty"`

	Hidden        bool `json:"hidden,omitempty"` // hide on every viewport
	HiddenMobile  bool `json:"hiddenMobile,omitempty"`
	HiddenDesktop bool `json:"hiddenDesktop,omitempty"`

	All     StyleBag `json:"all"`
	Mobile  StyleBag `json:"mobile"`
	Desktop StyleBag `json:"desktop"`

	// Legacy pre-responsive flat fields — folded into All on the next save.
	FontSize     string `json:"fontSize,omitempty"`
	FontWeight   string `json:"fontWeight,omitempty"`
	TextAlign    string `json:"textAlign,omitempty"`
	Color        string `json:"color,omitempty"`
	Background   string `json:"background,omitempty"`
	Padding      string `json:"padding,omitempty"`
	Margin       string `json:"margin,omitempty"`
	BorderWidth  string `json:"borderWidth,omitempty"`
	BorderColor  string `json:"borderColor,omitempty"`
	BorderRadius string `json:"borderRadius,omitempty"`
	Width        string `json:"width,omitempty"`
	MaxWidth     string `json:"maxWidth,omitempty"`
}

// The breakpoint the studio scopes Mobile / Desktop layers to.
const (
	MediaMobileMax  = "(max-width: 767.98px)"
	MediaDesktopMin = "(min-width: 768px)"
)

// TextReplacement swaps a piece of user-panel copy for another. Rendered as
// plain text (never HTML). Mode "exact" (default) replaces a text node whose
// trimmed content equals Find; "contains" does a substring replace.
type TextReplacement struct {
	Find    string `json:"find"`
	Replace string `json:"replace"`
	Mode    string `json:"mode,omitempty"` // "" | "exact" | "contains"
}

// AppearanceSections groups the structured overrides.
type AppearanceSections struct {
	Hero   HeroSection   `json:"hero"`
	Header HeaderSection `json:"header"`
}

// HeroSection overrides the signed-out landing hero of the user panel.
type HeroSection struct {
	Title        string `json:"title,omitempty"`
	Subtitle     string `json:"subtitle,omitempty"`
	Note         string `json:"note,omitempty"` // small line under the sign-in form
	ShowSubtitle *bool  `json:"showSubtitle,omitempty"`
	Align        string `json:"align,omitempty"`      // "" | start | center | end
	Background   string `json:"background,omitempty"` // css color / gradient
	Image        string `json:"image,omitempty"`      // data: URL
	MinHeight    string `json:"minHeight,omitempty"`  // e.g. "70vh"
}

// HeaderSection overrides the user-panel top bar.
type HeaderSection struct {
	Brand      string `json:"brand,omitempty"`
	Background string `json:"background,omitempty"`
}
