package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/dotinschool/dotinschool/internal/apperr"
	"github.com/dotinschool/dotinschool/internal/model"
	"github.com/dotinschool/dotinschool/internal/service"
)

// GET /api/admin/ai-status — the effective (resolved) AI configuration.
func (s *server) handleAIStatus(c *gin.Context) {
	cfg := s.svc.AI.ResolveConfig()
	resp := gin.H{
		"configured": cfg != nil && cfg.APIKey != "",
		"model":      firstNonEmpty(s.cfg.AIEnvModel, service.AIDefaultModel),
		"baseUrl":    firstNonEmpty(s.cfg.AIEnvBaseURL, service.AIDefaultBaseURL),
		"source":     nil,
		"label":      nil,
	}
	if cfg != nil {
		resp["model"] = cfg.Model
		resp["baseUrl"] = cfg.BaseURL
		resp["source"] = cfg.Source
		resp["label"] = cfg.Label
	}
	c.JSON(http.StatusOK, resp)
}

// maskedProvider is the safe shape for a provider row (key never leaves as text).
type maskedProvider struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	KeyMask string `json:"keyMask"`
	HasKey  bool   `json:"hasKey"`
}

func maskProviders(ps []model.AIProvider) []maskedProvider {
	out := make([]maskedProvider, 0, len(ps))
	for _, p := range ps {
		out = append(out, maskedProvider{
			ID: p.ID, Label: p.Label, BaseURL: p.BaseURL, Model: p.Model,
			KeyMask: service.MaskKey(p.APIKey), HasKey: p.APIKey != "",
		})
	}
	return out
}

// GET /api/admin/ai-settings — the panel provider list with masked keys, plus
// presets, the .env fallback status, and defaults.
func (s *server) handleGetAISettings(c *gin.Context) {
	st := s.svc.AI.SettingsSnapshot()
	c.JSON(http.StatusOK, gin.H{
		"activeId":  st.ActiveID,
		"providers": maskProviders(st.Providers),
		"presets":   service.AIPresets,
		"env": gin.H{
			"present": s.cfg.AIEnvAPIKey != "",
			"baseUrl": firstNonEmpty(s.cfg.AIEnvBaseURL, service.AIDefaultBaseURL),
			"model":   firstNonEmpty(s.cfg.AIEnvModel, service.AIDefaultModel),
		},
		"defaults": gin.H{"baseUrl": service.AIDefaultBaseURL, "model": service.AIDefaultModel},
	})
}

// PUT /api/admin/ai-settings — save the full list + active selection. A key is
// only replaced when the admin actually typed a new value (not the mask).
func (s *server) handlePutAISettings(c *gin.Context) {
	var body struct {
		ActiveID  string `json:"activeId"`
		Providers []struct {
			ID      string `json:"id"`
			Label   string `json:"label"`
			BaseURL string `json:"baseUrl"`
			Model   string `json:"model"`
			APIKey  string `json:"apiKey"`
		} `json:"providers"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	if len(body.Providers) > service.AIMaxProviders {
		fail(c, apperr.New(400, "TOO_MANY", "حداکثر "+strconv.Itoa(service.AIMaxProviders)+" سرویس می‌توانید ثبت کنید."))
		return
	}

	prev := map[string]model.AIProvider{}
	for _, p := range s.svc.AI.SettingsSnapshot().Providers {
		prev[p.ID] = p
	}

	cleaned := make([]model.AIProvider, 0, len(body.Providers))
	for _, p := range body.Providers {
		label := clip(strings.TrimSpace(p.Label), 60)
		baseURL := clip(strings.TrimRight(strings.TrimSpace(p.BaseURL), "/"), 300)
		mdl := clip(strings.TrimSpace(p.Model), 120)

		// Guard the classic footgun: a browser password manager autofilling the
		// panel login username into the "نام مدل" field (it sits right before
		// the API-key field). Saving that makes every AI call send
		// model="administrator" and the gateway answers "No available channel
		// for model administrator".
		if mdl != "" && strings.EqualFold(mdl, s.cfg.AdminUsername) {
			fail(c, apperr.New(400, "BAD_MODEL",
				"«نام مدل» با نام کاربری پنل یکی است — به احتمال زیاد مرورگر آن را خودکار پر کرده. نام مدل واقعی را وارد کنید (مثلاً gpt-4o-mini) یا این فیلد را خالی بگذارید."))
			return
		}

		typedKey := ""
		if k := strings.TrimSpace(p.APIKey); k != "" && !service.LooksLikeMask(k) {
			typedKey = clip(k, 400)
		}
		id := p.ID
		if id == "" {
			id = uuid.NewString()
		}
		apiKey := typedKey
		if apiKey == "" {
			if pr, ok := prev[id]; ok {
				apiKey = pr.APIKey
			}
		}
		if label == "" && baseURL == "" && mdl == "" && apiKey == "" {
			continue // wholly empty row
		}
		if baseURL != "" {
			u, err := url.Parse(baseURL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				lbl := label
				if lbl == "" {
					lbl = "یک سرویس"
				}
				fail(c, apperr.New(400, "BAD_URL", "آدرس پایه‌ی نامعتبر برای «"+lbl+"»: "+baseURL))
				return
			}
		}
		cleaned = append(cleaned, model.AIProvider{ID: id, Label: label, BaseURL: baseURL, Model: mdl, APIKey: apiKey})
	}

	activeID := body.ActiveID
	if activeID != "" && !containsProviderID(cleaned, activeID) {
		activeID = ""
	}
	if activeID == "" && len(cleaned) > 0 {
		activeID = cleaned[0].ID
		for _, p := range cleaned {
			if p.APIKey != "" {
				activeID = p.ID
				break
			}
		}
	}

	saved := model.AISettings{ActiveID: activeID, Providers: cleaned}
	if err := s.svc.Settings.SaveAISettings(saved); err != nil {
		s.failWith(c, err)
		return
	}

	st := s.svc.AI.SettingsSnapshot()
	c.JSON(http.StatusOK, gin.H{"activeId": st.ActiveID, "providers": maskProviders(st.Providers)})
}

// POST /api/admin/ai-settings/test — one real, tiny request against a saved
// provider (by id) or the current effective config.
func (s *server) handleTestAIProvider(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("ai-test", c.ClientIP(), 15, 10*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	_ = bindJSON(c, &body)

	var cfg *service.AIRuntimeConfig
	if body.ID != "" {
		var target *model.AIProvider
		for _, p := range s.svc.AI.SettingsSnapshot().Providers {
			if p.ID == body.ID {
				pp := p
				target = &pp
				break
			}
		}
		if target == nil || target.APIKey == "" {
			fail(c, apperr.New(400, "NO_KEY", "این سرویس هنوز کلید ذخیره‌شده ندارد — اول «ذخیره سرویس‌ها» را بزنید."))
			return
		}
		cfg = &service.AIRuntimeConfig{
			APIKey:  target.APIKey,
			BaseURL: strings.TrimRight(firstNonEmpty(target.BaseURL, service.AIDefaultBaseURL), "/"),
			Model:   s.svc.AI.ModelOrDefault(target.Model),
		}
	} else {
		cfg = s.svc.AI.ResolveConfig()
		if cfg == nil || cfg.APIKey == "" {
			fail(c, apperr.New(400, "AI_NOT_CONFIGURED", "سرویسی برای تست پیدا نشد."))
			return
		}
	}

	out, err := s.svc.AI.Chat([]service.ChatMessage{
		{Role: "user", Content: "فقط و دقیقاً این عبارت را بنویس: اتصال برقرار است."},
	}, cfg)
	if err != nil {
		s.failWith(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "model": out.Model, "sample": clip(out.Content, 120)})
}

var aizaRe = regexp.MustCompile(`^AIza[0-9A-Za-z_-]{10,}$`)

// POST /api/admin/ai-detect — identify which service an API key belongs to.
func (s *server) handleDetectAIProvider(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("ai-detect", c.ClientIP(), 30, 10*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}
	var body struct {
		APIKey string `json:"apiKey"`
	}
	_ = bindJSON(c, &body)
	key := strings.TrimSpace(body.APIKey)
	if key == "" || len([]rune(key)) < 12 || strings.HasPrefix(key, "•") {
		fail(c, apperr.InvalidInput("کلید معتبری وارد نشده است."))
		return
	}

	sig := func(label, baseURL, mdl string) gin.H {
		return gin.H{"method": "signature", "match": gin.H{"label": label, "baseUrl": baseURL, "model": mdl}}
	}
	switch {
	case strings.HasPrefix(key, "sk-or-"):
		c.JSON(http.StatusOK, sig("OpenRouter", "https://openrouter.ai/api/v1", "meta-llama/llama-3.1-8b-instruct:free"))
		return
	case regexp.MustCompile(`^gsk_[A-Za-z0-9]`).MatchString(key):
		c.JSON(http.StatusOK, sig("Groq", "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile"))
		return
	case strings.HasPrefix(key, "sk-ant-"):
		c.JSON(http.StatusOK, sig("Anthropic (سازگار با OpenAI)", "https://api.anthropic.com/v1", "claude-3-5-haiku-latest"))
		return
	case aizaRe.MatchString(key):
		c.JSON(http.StatusOK, sig("Gemini (سازگار با OpenAI)", "https://generativelanguage.googleapis.com/v1beta/openai", "gemini-2.0-flash"))
		return
	}

	// Generic "sk-..." — probe each candidate with a real GET /models.
	type cand struct{ label, baseURL, mdl string }
	candidates := []cand{
		{"OpenAI (ChatGPT)", "https://api.openai.com/v1", "gpt-4o-mini"},
		{"GapGPT", "https://api.gapgpt.app/v1", "gpt-4o-mini"},
		{"DeepSeek", "https://api.deepseek.com/v1", "deepseek-chat"},
	}
	var accepted []cand
	rejected := 0
	for _, cd := range candidates {
		r := s.svc.AI.Probe(cd.baseURL, key)
		if r.OK {
			accepted = append(accepted, cd)
		} else if r.Status == 401 || r.Status == 403 {
			rejected++
		}
	}
	if len(accepted) >= 1 {
		resp := gin.H{"method": "probe", "match": gin.H{
			"label": accepted[0].label, "baseUrl": accepted[0].baseURL, "model": accepted[0].mdl,
		}}
		if len(accepted) > 1 {
			labels := make([]string, 0, len(accepted))
			for _, a := range accepted {
				labels = append(labels, a.label)
			}
			resp["multiple"] = labels
		}
		c.JSON(http.StatusOK, resp)
		return
	}
	if rejected == len(candidates) {
		c.JSON(http.StatusOK, gin.H{"method": "probe", "match": nil, "reason": "rejected-everywhere"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"method": "probe", "match": nil, "reason": "unreachable"})
}

const reviewSystemPrompt = `تو یک دستیار ارشد بازبینی کد جاوا هستی. کل پاسخ را به زبان فارسی بده.
خطِ اولِ پاسخ باید دقیقاً و فقط همین باشد، بدون هیچ چیز دیگر روی آن خط:
AI_GENERATED_PERCENT: <عددی بین ۰ تا ۱۰۰>
این عدد تخمین تو از احتمالِ این است که این کد توسط یک ابزار هوش مصنوعی تولید شده باشد (۰ = مطمئناً انسان، ۱۰۰ = مطمئناً هوش مصنوعی). به این نشانه‌ها توجه کن: یکدستیِ بیش از حدِ سبک، نبودِ کدِ مرده یا تلاش‌های نافرجام، کامنت‌های توضیحیِ کتاب‌وار، نام‌گذاری بی‌نقص، الگوهای رایج مدل‌های زبانی.
از خط دوم به بعد، گزارش را با قالب Markdown و دقیقاً با همین سرفصل‌ها و ترتیب بنویس:
## خلاصه‌ی داوری
یکی از این چهار حالت را صریح بنویس: «درست و بی‌ایراد» / «درست ولی قابل بهبود» / «دارای ایراد منطقی» / «کامپایل نمی‌شود».
## آیا کامپایل و اجرا می‌شود؟
بله یا خیر برای کامپایل، و بله یا خیر برای اجرا شدن بدون خطای زمان اجرا. اگر خیر، خطاها را با شماره‌ی خط فهرست کن.
## خروجی برنامه
اگر متد main دارد، خروجی دقیق کنسول را داخل بلوک کد نشان بده. اگر برنامه ورودی می‌گیرد، یک ورودی نمونه فرض کن و صریح بگو چه فرض کردی. اگر main ندارد، رفتار متدهای اصلی را با یک مثال فراخوانی توضیح بده.
## توضیح گام‌به‌گام اجرا
مسیر اجرای برنامه را کوتاه و مرحله‌به‌مرحله شرح بده.
## احتمال تولید با هوش مصنوعی
همان عددِ خط اول را اینجا هم به‌صورت درصد بنویس و در ۲ تا ۴ جمله دلیلش را توضیح بده.
## باگ‌ها و ایرادها
هر مورد را با شماره‌ی خط و توضیح بنویس؛ اگر ایرادی نیست همین را بگو.
## پیچیدگی زمانی و حافظه
Big-O زمان و حافظه را بده.
## پیشنهاد بهبود
در صورت امکان یک نسخه‌ی اصلاح‌شده‌ی کوتاه در بلوک کد جاوا بگذار.
اگر جایی مطمئن نیستی صریح بگو. بیرون از بخش «پیشنهاد بهبود» کد جدیدی از خودت ننویس.`

var aiPercentRe = regexp.MustCompile(`(?i)^\s*AI[_ ]?GENERATED[_ ]?PERCENT\s*[:=]\s*(\d{1,3})\s*%?`)

// POST /api/admin/review-code — run the fixed review prompt over a snippet.
func (s *server) handleReviewCode(c *gin.Context) {
	if !s.svc.RateLimiter.Allow("ai-review", c.ClientIP(), 20, 10*time.Minute) {
		fail(c, apperr.TooManyRequests())
		return
	}
	var body struct {
		Filename string `json:"filename"`
		Code     string `json:"code"`
	}
	if err := bindJSON(c, &body); err != nil {
		fail(c, err)
		return
	}
	filename := clip(firstNonEmpty(strings.TrimSpace(body.Filename), "Main.java"), 120)
	if strings.TrimSpace(body.Code) == "" {
		fail(c, apperr.InvalidInput("کدی برای بررسی داده نشده — یک فایل .java انتخاب کنید یا کد را وارد کنید."))
		return
	}
	if len([]rune(body.Code)) > service.AIMaxCodeChars {
		fail(c, apperr.New(413, "CODE_TOO_LONG", "حجم کد بیش از حد مجاز است (سقف "+strconv.Itoa(service.AIMaxCodeChars)+" کاراکتر)."))
		return
	}

	user := "نام فایل: " + filename + "\n\n```java\n" + body.Code + "\n```"
	out, err := s.svc.AI.Chat([]service.ChatMessage{
		{Role: "system", Content: reviewSystemPrompt},
		{Role: "user", Content: user},
	}, nil)
	if err != nil {
		s.failWith(c, err)
		return
	}

	review := out.Content
	var aiScore *int
	if m := aiPercentRe.FindStringSubmatchIndex(review); m != nil {
		n, _ := strconv.Atoi(review[m[2]:m[3]])
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		aiScore = &n
		review = strings.TrimLeft(review[m[1]:], " \t\r\n")
	}

	c.JSON(http.StatusOK, gin.H{
		"review":  review,
		"aiScore": aiScore,
		"model":   out.Model,
		"usage":   jsonRawOrNull(out.Usage),
	})
}

// ---- small helpers --------------------------------------------------

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func containsProviderID(ps []model.AIProvider, id string) bool {
	for _, p := range ps {
		if p.ID == id {
			return true
		}
	}
	return false
}

func jsonRawOrNull(r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	return r
}
