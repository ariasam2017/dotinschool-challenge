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

// ElementOverride is one picked element and what to do with it. Every non-empty
// style field becomes a `selector { prop: value !important }` declaration.
type ElementOverride struct {
	Selector string `json:"selector"`
	Text     string `json:"text,omitempty"`
	SetText  bool   `json:"setText,omitempty"` // apply Text (even when empty)
	Hidden   bool   `json:"hidden,omitempty"`

	FontSize   string `json:"fontSize,omitempty"`   // e.g. "1.1rem", "18px"
	FontWeight string `json:"fontWeight,omitempty"` // "300".."900" | normal | bold
	TextAlign  string `json:"textAlign,omitempty"`  // right | left | center | justify
	Color      string `json:"color,omitempty"`      // text colour
	Background string `json:"background,omitempty"` // background colour / gradient

	Padding string `json:"padding,omitempty"` // CSS length shorthand
	Margin  string `json:"margin,omitempty"`

	BorderWidth  string `json:"borderWidth,omitempty"` // e.g. "1px"
	BorderColor  string `json:"borderColor,omitempty"`
	BorderRadius string `json:"borderRadius,omitempty"` // e.g. "12px"

	Width    string `json:"width,omitempty"`
	MaxWidth string `json:"maxWidth,omitempty"`

	Label string `json:"label,omitempty"` // human hint for the admin list
}

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
