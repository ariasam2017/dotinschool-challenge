# HANDOFF — dotinschool-challenge

> **نکته (۲۰۲۶-۱۰-۱۶):** این مخزن از پروژهٔ مشترک «dotinschool-Game&Challenge» جدا شد. بخش‌های این سند که به پروژهٔ دیگر مربوط است تاریخی‌اند؛ مرجع به‌روز `README.md` و `CLAUDE.md` است.

سندی برای شروعِ سریع یک نشستِ جدیدِ Claude روی یک سیستم دیگر، برای ادامهٔ همین پروژه. تاریخ اولین نگارش: ۲۰۲۶-۰۹-۰۵ · آخرین به‌روزرسانی: ۲۰۲۶-۰۹-۰۸.

## ۰. اولین قدم در نشست جدید

این پیام را اولین پیام بدهید (پروژه باید از قبل clone شده باشد — بخش «۲. دسترسی به مخزن» را ببینید):

```
پروژه‌ای در فلان-مسیر هست. اول فایل HANDOFF.md را کامل بخوان، بعد README.md و
git log --oneline -30 و ساختار کد (cmd/, internal/, web/src/, scripts/, build/)
را مرور کن. بعدش منتظر بمان تا بگویم چه کاری انجام بدهی.
```

## ۱. پروژه چیست

`dotinschool-challenge` — سامانهٔ آنلاین چالش برنامه‌نویسی، با معماری‌ای مشابه پنل‌های **x-ui / 3x-ui**: یک باینری استاتیک تک‌فایل به زبان **Go** که هم فرانت‌اند Vue بیلدشده را داخل خودش embed می‌کند و هم یک REST API کوچک روی SQLite دارد. دو پنل مجزا: **پنل شرکت‌کننده** و **پنل مدیریت**. جزئیات کامل فنی (پشته، معماری، قرارداد API، امنیت) توی README.md است — این‌جا تکرارش نمی‌کنم.

## ۲. دسترسی به مخزن

مخزن **Private** است: `https://github.com/ariasam2017/dotinschool-challenge` (صاحبِ اکانت گیت‌هاب: `ariasam2017`).

برای کلون کردن روی سیستم جدید، یک **fine-grained Personal Access Token** لازم است (مراحل کامل ساختنش توی README.md بخش «نصب سریع» هست: Resource owner `ariasam2017`، Only select repositories → `dotinschool-challenge`، Repository permissions → Contents: Read-only). توکن قبلی که حین این پروژه یک‌بار توی یک اسکرین‌شات لو رفت **Revoke شده** — اگر نشده، حتماً همین الان Revoke کنید و یک توکن تازه بسازید.

```bash
git clone https://YOUR_TOKEN@github.com/ariasam2017/dotinschool-challenge.git
```

اگر سیستم جدید هم پشت شبکهٔ فیلترشده است (نکتهٔ ۶ را ببینید)، شاید همین `git clone` هم گیر کند.

## ۳. پشتهٔ فنی (خلاصه — کامل توی README.md)

- بک‌اند: Go 1.22+ · Gin · GORM + SQLite (pure-Go) · robfig/cron · logrus
- فرانت‌اند: Vue 3 + Ant Design Vue 4 + Vite · RTL کامل · فارسی · تقویم جلالی
- ادیتور: CodeMirror 6 با Live Template های سبک IntelliJ
- استقرار: `scripts/install.sh` (bare-metal + systemd) و Docker (`build/`, `docker-up.ps1`, `docker-up.sh`)

## ۴. کارهای نشست‌های بعدی (۲۰۲۶-۰۹-۰۶ تا ۰۹-۰۸) — جدیدترین و مهم‌ترین

### ۴.۱ فونت
- فونت رابط از **Vazirmatn** به **IRANYekanX** عوض شد (۶ وزن، woff2). فایل‌ها در `web/src/assets/fonts/`، تعریف در `web/src/styles/fonts.css`، توکن `--font-ui` در `tokens.css`، و `fontFamily` تم Ant در `web/src/lib/theme.ts`. `web/assets/` (بیلد کامیت‌شده) هم بازساخته شد.

### ۴.۲ اعداد فارسی سراسری
- `web/src/lib/fa-digits.ts` (جدید) + یک خط در `web/src/main.ts`: یک `MutationObserver` روی `document.body` که همهٔ ارقام لاتینِ متن‌ها و placeholderها را در **هر دو پنل** (و منوهای بازشوی Ant که به `body` پورتال می‌شوند) به فارسی می‌کند. **استثناها:** `.cm-editor`، `code`/`pre`، `input`/`textarea` (وگرنه سابمیتِ کد جاوا/شماره خراب می‌شود؛ مسیر ارسال هم با `toEn` نرمال می‌شود).

### ۴.۳ آدرسِ استارت‌آپ واقعی (به‌جای 127.0.0.1)
- `internal/config/config.go`: فیلدهای `Domain / PublicHost / PublicPort / PublicURL` + متد `PublicBaseURL()` + `outboundIP()` (تشخیص IP کارت شبکه، در Docker خاموش). اولویت: `PUBLIC_URL` > `DOMAIN` (+ پورت غیراستاندارد) > `PUBLIC_HOST` > IP خودکار > `localhost`.
- `cmd/server/main.go`: به‌جای هاردکدِ 127.0.0.1 از `cfg.PublicBaseURL()` استفاده می‌کند + `var version = "dev"` که با `-ldflags -X main.version=` استمپ و در استارت‌آپ لاگ می‌شود.
- `build/docker-install.sh` و `docker-up.ps1` مقدار `PUBLIC_HOST` را در `.env` می‌نویسند.

### ۴.۴ سخت‌سازیِ `dotinschool-challenge update`
- اکشنِ `update`/`rebuild` در wrapperِ CLIِ داخل `build/docker-install.sh`: اگر `root` نباشد با `sudo` **دوباره اجرا می‌شود**؛ مراحل شماره‌دار با خطای صریح (نه زنجیرهٔ `&&` بی‌صدا)؛ خطای «رفرش CLI» غیرِمرگبار؛ `compose up -d --force-recreate`؛ آخرش هش گیت و نسخهٔ در حال اجرا را چاپ می‌کند.
- `docker-up.ps1` هم `--force-recreate` می‌زند، `main.version` را از `git describe` استمپ می‌کند، و خط `deployed build: <hash>` را چاپ می‌کند.

### ۴.۵ متنِ صفحهٔ ورودِ پنل کاربر
- در `web/src/views/user/UserApp.vue`: پاراگرافِ «کد جاوای خود را…» حذف شد؛ تیتر شد «به چالش برنامه‌نویسی مدرسه داتین خوش آمدید»؛ هیرو **تک‌ستونی** شد (تیتر بالا، فرم زیرش، `max-width` ~۴۸۰px، وسط‌چین). تیتر و متنِ برند حالا override-aware اند (از Design Studio).

### ۴.۶ Design Studio — بخش «ظاهر و طراحی» در پنل ادمین (بزرگ‌ترین کار)
ویرایشگرِ ظاهریِ زندهٔ پنل کاربر، بدون rebuild/تغییرِ سورس در زمان استفاده.

**بک‌اند:**
- `internal/model/appearance.go` (جدید): `Appearance{ tokens, dark, sections{hero,header}, customCss, texts[], overrides[] }`، `StyleBag`، `ElementOverride{ selector, setText, text/textMobile/textDesktop, hidden/hiddenMobile/hiddenDesktop, all/mobile/desktop: StyleBag }`، `TextReplacement`. کلیدهای setting: `appearanceDraft` / `appearancePublished`.
- `internal/service/appearance.go` (جدید): متدهای `AppearanceDraftRaw/PublishedRaw`, `SaveAppearanceDraft`, `PublishAppearance`, `RevertAppearanceDraft`, `ResetAppearance` + اعتبارسنجیِ سخت (whitelist توکن‌ها، charset مقادیر و سلکتور، بدون `; { } @ url(http)`، تصویر ≤۵۱۲KB، customCss ≤۲۰KB با بلاکِ `@import`/script/expression).
- `internal/web/handlers_appearance.go` (جدید) + روت‌های `GET/PUT /api/admin/appearance` و `POST …/publish|revert|reset` در `server.go`.
- `internal/service/settings.go`: فیلد `Appearance` به `ConfigDTO` اضافه شد → `GET /api/config` عمومی نسخهٔ منتشرشده را حمل می‌کند (پنل کاربر هر ۱ ثانیه poll می‌کند، پس انتشار ظرف ~۱ ثانیه همه‌جا).
- `internal/database/seed.go`: نصب تازه از `internal/database/appearance.default.json` (کامیت‌شده، فعلاً `{}`) seed می‌شود — جای گذاشتن یک طرحِ پیش‌فرض برای پروژه.
- `internal/web/middleware.go`: **CSP سست شد** — `frame-ancestors 'none' → 'self'` و `X-Frame-Options: DENY → SAMEORIGIN` (برای iframeِ پیش‌نمایشِ هم‌مبدأ). فریمِ cross-origin همچنان مسدود.

**فرانت‌اند:**
- `web/src/lib/appearance.ts` (جدید): `buildCss(a, force?)` یک `<style id="dotin-appearance">` می‌سازد؛ observerِ متن (find→replace و override هر عنصر، viewport-aware)؛ انتخابگرِ کلیکی (`cssPath`، `textLeaf` که به عنصرِ متن‌دار پایین می‌رود)؛ پلِ postMessage برای `?preview=1` (پیام‌های `appearance` / `device` / `pick` / `retarget`).
- `web/src/views/admin/AppearanceSection.vue` (جدید): سه تب —
  - **تنظیمات**: توکن‌های رنگِ روشن/تیره، گِردی، فونت، هیرو، هدر (همهٔ آکاردئون‌ها از اول بسته).
  - **عناصر**: «انتخاب عنصر از پیش‌نمایش» → روی هر عنصر کلیک → ویرایشگرش. کنترل‌ها: نمایش/مخفی (اسکوپ‌دار: مخفی موبایل/دسکتاپ)، تغییر متن، اندازه فونت (دکمهٔ `−`/`+` + کلیدهای `+`/`-`/`↑`/`↓` روی فیلد، واحد ثابت می‌ماند)، ضخامت، تراز، رنگ متن/پس‌زمینه، padding/margin، حاشیه، width/max-width. دکمه‌های «↑ والد / ↓ فرزند» برای اصلاحِ هدف. زیرِ ویرایشگر: «CSSِ تولیدشده برای این عنصر» (فقط‌خواندنی). فقط عنصرِ در حال ویرایش نشان داده می‌شود (لیستِ همیشگی ندارد)؛ حذف فقط ته ویرایشگر با تأیید + نوار «بازگرداندن».
  - **CSS سفارشی**: CSS خام (sanitize‌شده).
  - نوار ابزار: ذخیره پیش‌نویس / انتشار / برگردان / پیش‌فرض + **خروجی/ورودی JSON** برای انتقالِ طرح بین نمونه‌ها. چیپِ `build <زمان>` (از `vite define: __BUILT_AT__`) برای تشخیصِ نسخهٔ در حال اجرا.
- **اسکوپ ریسپانسیو**: هر override دو لایه دارد — **موبایل** (`@media max-width:767.98px`) و **دسکتاپ** (`@media min-width:768px`). لایهٔ دیتاییِ `all` هنوز در مدل هست ولی **از UI برداشته شد** (کاربر فعلاً نمی‌خواهد). دکمهٔ «تغییرات برای: موبایل | دسکتاپ»؛ فیلدِ خالی = بدون تغییر.
- **پیش‌نمایش**: iframe کادر را پر می‌کند؛ ادمین با پیام `{__dotin:'device'}` می‌گوید کدام viewport شبیه‌سازی شود و `buildCss(a, force)` رولِ آن لایه را **بدون `@media`** می‌سازد تا در هر عرضی دیده شود (بدون zoom/scale/scrollbar — این‌ها قبلاً امتحان شدند و شکستند).
- `web/src/stores/event.ts`: لایهٔ منتشرشده را روی پنل کاربر اعمال می‌کند (فقط `IS_USER` و نه وقتی iframeِ پیش‌نمایش استایل را کنترل می‌کند).
- `web/src/api/types.ts` + `client.ts`: تایپ‌ها و توابعِ `getAppearance/saveAppearanceDraft/publishAppearance/revertAppearance/resetAppearance`.
- `web/src/components/AppIcon.vue`: آیکون‌های `palette`, `minus`, `chevron-up` اضافه شد.

**نکتهٔ باز:** کاربر گزارش کرد ادیتِ لایهٔ موبایل/دسکتاپ گاهی «فقط روی موبایل» اعمال می‌شود. منطقِ `buildCss` جداگانه تست شد و **درست است**؛ رفعِ نهایی نیاز به دیدنِ اسکرین‌شاتِ «CSSِ تولیدشده» + کنسولِ `[dotin-preview]` از سیستمِ کاربر دارد. اغلبِ این «هنوز خرابه»ها به‌خاطرِ ری‌دیپلوی‌نکردن یا کشِ مرورگر بوده.

### ۴.۷ بازطراحیِ پنل ادمین با `ui-ux-pro-max` — دو پاس، فقط لوکال (push‌نشده)

هدف: طرحِ سبزِ برند `#006839` + دارک‌مودِ حوزهٔ تکنولوژی، و بعد یک **دیزاینِ ساختاریِ نو** برای پنل ادمین (نه صرفاً رنگ). اسکیل در `.claude/skills/ui-ux-pro-max/` (خارج از گیت). خروجی اسکیل: سبکِ `data-dense-dashboard` + الگوی `real-time-operations`.

- **پاس ۱ — رنگ‌بندی (کامیت `7384940`):** اسکیلِ سبزِ `tokens.css` روی `#006839` (`--green-800` هرگز عوض نشود؛ `--green-400: #3ccf8d` برای متن/لینک در دارک). توکنِ `--primary-bright` (روشن: `#006839`، دارک: `#3ccf8d`). هر دو بلاکِ دارک بازنویسی و یکسان: زمینهٔ `#0a1310`، سطوح `#101914/#16221c/#1f2f27`، هیرلاین‌ها با ته‌رنگِ سبز، وضعیت‌ها (amber/red/blue) متمایز. override ایندیگوی `[data-panel='admin']` حذف شد. `theme.ts`: توکن‌های antd هم‌راستا. ۷ استفادهٔ متنیِ `var(--primary)` به `var(--primary-bright)` تغییر کرد. رنگِ outlineِ انتخابگرِ استودیو → `#34c98b`.
- **پاس ۲ — ساختار (کامیت `7fe6ff1`):**
  - `tokens.css`: بلاکِ `:root[data-panel='admin']` — تراکمِ داشبورد: `--space-5/6/7` کوچک‌تر، `--header-h: 52px`، `--sidebar-w: 236px`، `--rail-w: 64px`، `--content-max: 1440px`. فقط فاصله؛ رنگ/روشنایی دست‌نخورده.
  - `AdminShell.vue` (بازنویسی): نویگیشنِ کناری از لیستِ تختِ ۸تایی → **۴ گروهِ برچسب‌دار** (رویداد / کاربران / هوش مصنوعی / سامانه). آیتمِ فعال میلهٔ لبه‌ایِ `--primary` می‌گیرد (نه فقط تینت). **ریلِ آیکونیِ جمع‌شونده ۶۴px** با کلید در ردیفِ برند و در تاپ‌بار، ذخیره در `localStorage` (`dotinschool-challenge_admin_rail`). تاپ‌بار ۶۰→۵۲px، حالا نوارِ زمینه: بردکرامبِ «پنل مدیریت / <بخش>» + نشانگرِ ضربانیِ «زنده».
  - `StatCard.vue` (بازنویسی): کاشیِ ابزارگونه — برچسب بالا-راست، چیپِ آیکون بالا-چپ، عددِ `2rem`، نوارِ رنگِ tone روی لبهٔ بالا، اسپارک‌لاین چسبیده به لبه.
  - `base.css`: `.card-hd` (هدرِ کارت با هیرلاینِ تمام‌عرض، از padding بیرون می‌زند) و `.data-table` (هدرِ چسبان، ارتفاعِ ردیفِ ثابت، hover) برای یکدستیِ جدول‌های همهٔ بخش‌ها؛ انیمیشنِ `admin-rise` (ظاهرشدنِ پلکانیِ سکشن‌ها، پشتِ `prefers-reduced-motion`). `.section-title` کمی پررنگ‌تر (`--text-dim`, ۷۰۰).
  - `OverviewSection.vue`: تیترها با `.card-hd`، جدولِ «آخرین شرکت‌کنندگان» با `.data-table`.
- **پاس ۳ — دارک‌مودِ خنثیِ ادمین (کامیت `acf851b`):** به درخواستِ کاربر «رنگ دارک‌مود پنل ادمین را مثل پنل ثنایی کن». `tokens.css`: بلاک‌های `DARK / ADMIN` در انتهای فایل — فقط برای `[data-panel='admin']` (هم `@media prefers-color-scheme:dark :not([data-theme='light'])` هم `[data-theme='dark'][data-panel='admin']`، اسپسیفیسیتیِ بالاتر از بلاک‌های عمومیِ دارک). ته‌رنگِ زمردی برداشته شد؛ استکِ خاکستریِ خنثیِ سبکِ Ant/3x-ui: `--bg:#141414`، `--surface:#1d1d1d`، `--surface-2:#262626`، `--surface-3:#303030`، متن `#ededed/#a6a6a6/#6f6f6f`، هیرلاین `rgba(255,255,255,.11/.19)`، `--code-bg:#171717`. فقط سطوح/متن/هیرلاین override می‌شوند؛ `--primary` (سبزِ `#006839`)، `--primary-bright` (`#3ccf8d` برای متن/لینک)، وضعیت‌ها، سایه‌ها و `--ring` از بلاک‌های عمومیِ دارک cascade می‌شوند. `theme.ts`: `antdTheme` حالا `admin = APP_MODE==='admin'` را جدا می‌کند — `darkBg`/`darkText`/`darkBorder` برای ادمین خاکستریِ خنثی، برای کاربر همان مشکی-زمردی. **دارک‌مودِ پنل کاربر دست‌نخورده ماند.**
  - پاس‌های بعدی (هنوز نشده): `ParticipantsSection` / `ClientErrorsSection` جدول‌ها به `.data-table`؛ پالیشِ `Schedule/Challenge/AiReview/CodeAssistant`؛ `Gauge`/`MiniAreaChart`.
- **جریانِ بازبینی:** هر پاس → کامیتِ لوکال → `git bundle create ../design-redesign.bundle 885e541..HEAD` → کاربر روی داکر می‌آورد بالا و بررسی می‌کند → **push فقط با اجازهٔ صریحِ کاربر**.

## ۴٫الف. کارهای نشست اول (۲۰۲۶-۰۹-۰۵) — README، اسکریپت‌های نصب، ویندوز

۱. **README.md کامل بازنویسی شد**: حرفه‌ای، بدون اشاره به تاریخچهٔ Node.js قبلی، با wrapper های `<div dir="rtl" align="right">` دور هر بلاک فارسی برای راست‌چین‌شدنِ درست، و بلاک‌های کد (```bash/powershell) عمداً **بیرون** آن divها تا جهتشان LTR بماند. هر جا عدد نسخهٔ لاتین (مثل `Go 1.22+`) وسط متن فارسی بود، یک RLM نامرئی بعدش گذاشته شد تا جهت‌متن بهم نریزد.

۲. **مخزن روی گیت‌هاب ساخته و push شد** (Private). پیام‌های کامیت به سبک Conventional Commits انگلیسی هستند (`feat/fix/docs/refactor/build/chore`) — تاریخچهٔ اولیه هم یک‌بار با `git filter-branch` به همین سبک بازنویسی و force-push شد (چون مخزن تازه بود و فقط مالِ خودِ کاربر).

۳. **اسکریپت نصب لینوکس (`scripts/install.sh`)** به سبک x-ui/Sanaei بازنویسی و توسعه یافت:
   - همهٔ پیام‌ها انگلیسی، منوی جعبه‌ای تعاملی.
   - چون مخزن Private است، توکن گیت‌هاب را **خودش تعاملی می‌پرسد** (`ensure_gh_token`) — نه اینکه توی دستور جای‌گذاری بشود. توکن فقط همان یک‌بار لازم است؛ داخل remote گیت کلون محلی ذخیره می‌شود.
   - `dotinschool-challenge cert` (SSL از راه HTTP-01) و `dotinschool-challenge cert-cf` (SSL از راه Cloudflare DNS، برای وقتی پورت ۸۰ باز نیست) — موقع نصب هم اگر دامنه داشته باشید همان لحظه ازتان می‌پرسد و SSL کامل فعال می‌کند.
   - `dotinschool-challenge log` (لاگ زنده/پاک‌کردن)، `dotinschool-challenge reset` (بازنشانی کامل به پیش‌فرض — مخرب، تأیید می‌خواهد)، `dotinschool-challenge fail2ban` + `dotinschool-challenge banlog` (بن خودکار IP بعد از چند بار ورود اشتباه به پنل ادمین).
   - `dotinschool-challenge warp` — **مهم**: بعضی شبکه‌ها (خیلی از سرورهای ایران) گیت‌هاب را کامل روی IP بلاک می‌کنند (نه فقط DNS — خودِ TCP تایم‌اوت می‌شود). این دستور Cloudflare WARP را نصب و وصل می‌کند که این بلاک را دور می‌زند. **این مشکل واقعاً روی VPS کاربر (لینوکسِ ایران) پیش آمد و با WARP حل شد.**
   - `build/docker-install.sh` (نصب Docker روی لینوکس) هم همین رفتارِ توکنِ تعاملی و منطقِ WARP را دارد؛ اگر Docker نصب نباشد **خودش نصبش می‌کند**.

۴. **نصب ویندوز** (`scripts/windows-install.ps1` + `docker-up.ps1`) — این بخش پر از رفع‌باگ بود، دقت کنید:
   - `windows-install.ps1` هم توکن را تعاملی می‌پرسد (`Ensure-GhToken`, ورودی مخفی)، مخزن را در `%USERPROFILE%\dotinschool-challenge` کلون/آپدیت می‌کند، بعد `docker-up.ps1` را صدا می‌زند.
   - **باگ حل‌شده ۱ (Execution Policy)**: چون `windows-install.ps1` از راه `irm | iex` اجرا می‌شود ولی `docker-up.ps1` را به‌عنوان فایل واقعی صدا می‌زند، ویندوز با «running scripts is disabled» رد می‌کرد. با `Set-ExecutionPolicy -Scope Process -Bypass -Force` (فقط برای همان پردازش، بدون نیاز به Admin) حل شد.
   - **باگ حل‌شده ۲ (پنجره ناگهان بسته می‌شد)**: تابع `Die` هر دو اسکریپت از `exit 1` استفاده می‌کرد. چون این اسکریپت‌ها مستقیم توی نشستِ زندهٔ کاربر اجرا می‌شوند (نه یک پردازش جدا)، `exit` کل پنجرهٔ PowerShell را می‌بست، نه فقط اسکریپت را — یعنی هر خطایی (هرچی بود) پنجره را بی‌صدا می‌بست. با تغییر `exit 1` به `throw $m` حل شد (تست شد: throw فقط اسکریپت را متوقف می‌کند، نشست زنده می‌ماند).
   - روی ویندوز، **Go خودش نصب می‌شود** (مستقیم از go.dev/dl، نه از winget — چون manifest جامعهٔ winget برای `GoLang.Go` به نسخهٔ ۱.۲۷.۰ اشاره می‌کرد که اصلاً منتشر نشده بود، ۴۰۴ می‌داد). **Docker Desktop عمداً خودکار نصب نمی‌شود** — چون نصبش تعاملی است و ممکن است ری‌استارت برای WSL2 بخواهد؛ اگر نصب نباشد فقط پیام انگلیسی می‌دهد که نصبش کنید و دوباره اجرا کنید.
   - **باگ ۳ (دانلود Go شکست می‌خورد) — رفعِ اول ناقص بود، الان اصلاح شده**: اولین حدسم این بود که فقط نسخهٔ پین‌شده (۱.۲۷.۱) مشکل داشت (متادیتای go.dev زودتر از آپلود کامل فایل به‌روز شده). آن رو با گرفتن لیست نسخه‌های واقعی از `https://go.dev/dl/?mode=json` رفع کردم. **ولی بعد از تست دوباره، کاربر گزارش داد که ۵ نسخهٔ متفاوت (۱.۲۷.۰، ۱.۲۶.۸، ۱.۲۶.۷، ۱.۲۶.۶، ...) همه با همون الگو شکست خوردند: `go.dev` همیشه ۴۰۴، `golang.google.cn` همیشه ۴۰۳.** این الگو (شکست یکسان روی هر نسخه‌ای) یعنی مشکل واقعاً شبکه‌ایه، نه نسخه — به‌احتمال زیاد `dl.google.com` هم مثل گیت‌هاب (نکتهٔ بالا) روی شبکهٔ این کاربر فیلتر است، فقط این‌بار به‌جای تایم‌اوت، یک ۴۰۴ ساختگی برمی‌گرده (رفتار متفاوتِ فیلترینگ، ولی همون خانواده مشکل).
     **فیکس فعلی (تأییدنشده — آخرین کاری که باید انجام بدید همینه)**: به `docker-up.ps1` یک مسیر دستی اضافه کردم — اگر کاربر خودش با مرورگر فایل `go*.windows-amd64.zip` رو از https://go.dev/dl/ دانلود کنه و بریزه توی پوشهٔ Downloads (یا کنار خودِ `docker-up.ps1` با اسم `go-windows-amd64.zip`)، اسکریپت خودش پیدا و استفاده‌اش می‌کنه — بدون هیچ تماس شبکه‌ای. پیام خطای نهایی هم صراحتاً Cloudflare WARP برای ویندوز (https://1.1.1.1/) رو به‌عنوان راه‌حل اصلی پیشنهاد می‌ده (چون WARP دقیقاً همین مشکل رو برای گیت‌هاب روی VPS لینوکسی حل کرده بود). **از کاربر بپرسید کدوم راه رو امتحان کرد و جواب داد یا نه** — اگر WARP جواب داد، ارزش داره یک `install_warp` معادل برای ویندوز هم به اسکریپت اضافه بشه (فعلاً فقط دستی/لینوکسی پیاده شده).

۵. **امنیت**: یک خط لاگ WARN همیشه‌فعال (نه فقط با LOG_LEVEL=debug) به `internal/web/handlers_admin.go` اضافه شد برای هر تلاش ناموفق ورود ادمین، با فرمت ثابت `msg="admin login failed" ip=<IP>` تا fail2ban بتواند رویش match کند.

## ۵. نکات محیطیِ حیاتی

- **ماشینِ توسعه (ویندوز) حالا Go دارد.** ولی buildِ **بومیِ** ویندوز روی درایور `modernc.org/sqlite` شکست می‌خورد (پیش‌فرض، ربطی به تغییراتِ ما ندارد) — همیشه برای هدفِ واقعی cross-compile کنید:
  ```bash
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null ./cmd/server
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o /dev/null ./internal/web/ ./internal/service/
  ```
- **بیلدِ فرانت‌اند حافظه می‌خواهد.** روی این ماشین `vue-tsc`/`vite` وقتی RAM آزاد < ~۲GB بود OOM شد. با `NODE_OPTIONS="--max-old-space-size=2560" npm run build` و بستنِ مرورگر/کانتینر حل شد. `wsl --shutdown` هم ~۲GB آزاد می‌کند.
- **گردشِ کارِ تست:** کامیتِ لوکال → `git bundle create ../design-studio-full.bundle <last-pushed>..HEAD` → کاربر روی سرور `sudo git pull --ff-only /root/<bundle> HEAD && sudo dotinschool-challenge rebuild`. روی داکرِ ویندوزِ خودِ کاربر فقط `.\docker-up.ps1` (یا تابعِ `dsup` که در `$PROFILE` ساخته). بعدِ هر دیپلوی `Ctrl+Shift+R` لازم است؛ خطِ `deployed build:` و چیپِ `build <زمان>` در استودیو برای تأییدِ نسخه.
- کاربر (`amir`, `amir.kingstars@gmail.com`) گاهی پشت شبکهٔ فیلترشده کار می‌کند (VPS لینوکسیِ ایران تأیید شد؛ سیستم ویندوزی هنوز نامشخص). هر مشکل «اتصال گیر می‌کند / تایم‌اوت می‌شود بدون خطای واضح» را اول از این زاویه بررسی کنید.
- تمام اسکریپت‌های نصب (`scripts/install.sh`, `build/docker-install.sh`, `scripts/windows-install.ps1`) توکن گیت‌هاب را **تعاملی** می‌پرسند اگر از قبل نباشد؛ کد نمونهٔ دستورهای دقیق توی README.md «نصب سریع» است.
- `docker-up.ps1` و `scripts/windows-install.ps1` باید **ASCII خالص** بمانند (کامنت بالای هر فایل دلیلش رو توضیح می‌ده: PowerShell 5.1 بدون BOM فایل غیر-ASCII رو درست نمی‌خونه). قبل از هر commit روی این دو فایل، چک کنید:
  ```bash
  grep -nP '[^\x00-\x7F]' docker-up.ps1 scripts/windows-install.ps1
  ```
- README.md را اگر ویرایش کردید، تعادل divها را چک کنید (باید مساوی باشند) و مطمئن شوید هیچ کاراکتر غیرASCII داخل بلاک‌های کد نرفته:
  ```bash
  grep -c '<div dir="rtl"' README.md; grep -c '</div>' README.md
  awk '/^```/{f=!f;next} f{print}' README.md | grep -nP '[^\x00-\x7F]'
  ```

## ۶. محدودیت‌ها و ترجیحات کاربر

- کاربر فارسی می‌نویسد و ترجیح می‌دهد پاسخ‌ها هم فارسی و کوتاه باشند.
- پیام‌های کامیت: انگلیسی، سبک Conventional Commits. **از ۲۰۲۶-۰۹-۰۶ به بعد خطِ `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` انتهای هر پیامِ کامیت گذاشته می‌شود** (طبق راهنماییِ attributionِ نشست؛ کامیت‌های قدیمی‌تر این خط را ندارند).
- **قاعدهٔ صریح و مهم (۲۰۲۶-۰۹-۰۸): هیچ‌وقت بدون اجازهٔ صریحِ همان‌لحظهٔ کاربر `git push` نکنید.** یک «push» در پیامِ قبلی مجوزِ دائمی نیست. کار را لوکال commit کنید و برای تستِ داکر bundle بدهید؛ push فقط با دستورِ مستقیمِ «push کن روی گیت‌هاب». (این جایگزینِ حرفِ قبلیِ «معمولاً بدون پرسیدن push کنید» است.)
- force-push یا هر عملیاتِ مخرب: حتماً تأییدِ صریح.
- **HANDOFF.md را زنده نگه دارید**: بعد از تغییرِ واقعیِ کد/اسکریپت، همین فایل را هم به‌روز کنید و در همان دسته کامیت کنید (نه لزوماً یک کامیتِ جدا). push فقط طبق قاعدهٔ بالا.

## ۷. دستورهای مفید

```bash
git log --oneline -30                 # تاریخچهٔ اخیر
gh api repos/ariasam2017/dotinschool-challenge/commits --jq '.[0:10][].commit.message | split("\n")[0]'
bash -n scripts/install.sh build/docker-install.sh   # syntax check شل

# build و تأییدِ صحت (این ماشین)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null ./cmd/server
gofmt -l internal/ cmd/
( cd web && NODE_OPTIONS="--max-old-space-size=2560" npm run build )   # vue-tsc + vite

# انتقال به داکرِ کاربر (بدون push)
git bundle create ../design-studio-full.bundle <last-pushed-hash>..HEAD
```

## ۸. Design Studio — مرجعِ سریع

- **فایل‌های کلیدی:** `internal/model/appearance.go`، `internal/service/appearance.go`، `internal/web/handlers_appearance.go`، `web/src/lib/appearance.ts`، `web/src/views/admin/AppearanceSection.vue`.
- **جریانِ داده:** ادمین → `PUT /api/admin/appearance` (draft) → `POST …/publish` → ردیفِ `appearancePublished` در جدولِ `settings` → `GET /api/config` → `stores/event.ts` → `applyAppearance()` → `<style id="dotin-appearance">` روی `#app` پنل کاربر.
- **پیش‌نمایش:** iframeِ `/?preview=1`؛ ادمین با `postMessage` پیام‌های `appearance` / `device` / `pick` / `retarget` می‌فرستد؛ `initAppearanceBridge()` در همان فایل جواب می‌دهد.
- **دیباگ:** کنسولِ داخلِ iframe خطِ `[dotin-preview] device=… <css>` می‌زند؛ در استودیو زیرِ هر عنصر «CSSِ تولیدشده برای این عنصر» هست؛ چیپِ `build <زمان>` نسخه را نشان می‌دهد.
