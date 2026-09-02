# dotinschool — سامانهٔ چالش برنامه‌نویسی

بازنویسی کامل محصول Node.js روی پشتهٔ فنی پنل **x-ui / 3x-ui**: یک باینری استاتیک تک‌فایل به زبان **Go** که هم فرانت‌اند Vue بیلدشده را داخل خودش embed می‌کند و هم یک REST API کوچک روی SQLite دارد. دو پنل مجزا: **پنل شرکت‌کننده** و **پنل مدیریت**.

- بک‌اند: Go ۱.۲۲+ · Gin · GORM + SQLite (درایور pure-Go، بدون CGO) · robfig/cron · logrus · `go:embed`
- فرانت‌اند: Vue 3 + Ant Design Vue 4 + Vite · RTL کامل · فارسی · ارقام فارسی · تقویم جلالی · فونت Vazirmatn + JetBrains Mono
- ادیتور کد: CodeMirror 6 (های‌لایت جاوا، شماره خط، اتوکامپلیت، تطبیق پرانتز، Find/Replace، لینت سمت کلاینت)
- استقرار: `scripts/install.sh` + systemd + دستور سراسری `dotinschool` · Docker (Dockerfile چندمرحله‌ای + compose + override پروداکشن با Caddy برای HTTPS خودکار)

روی **IP خام و HTTP ساده** بدون هیچ خطا کار می‌کند؛ اگر مسیر cert/key ست شده باشد مستقیم TLS می‌دهد.

---

## اجرای محلی (توسعه)

پیش‌نیاز: Go ۱.۲۴+ و Node ۲۰+.

```bash
# ۱) بیلد فرانت‌اند (خروجی در web/assets/ که go:embed می‌شود)
cd web && npm install && npm run build && cd ..

# ۲) اجرای سرور
go run ./cmd/server
```

- پنل کاربر:  <http://127.0.0.1:8080/>
- پنل ادمین:  <http://127.0.0.1:8080/admin/>  (روی نصب لینوکسی، مسیر مخفی تصادفی)
- رمز ادمین: اگر `ADMIN_PASSWORD` در `.env` نباشد، یک رمز تصادفی ساخته، در `.env` ذخیره و در لاگ چاپ می‌شود.

### حالت توسعهٔ داغ (hot-reload فرانت)

```bash
make dev      # سرور Go روی :8080 + دیو‌سرور Vite روی :5173 با پروکسی /api
```

### دادهٔ دمو

```bash
go run ./cmd/seed     # پاک‌سازی و پرکردن دیتابیس با ~۱۶ شرکت‌کننده و یک رویداد زندهٔ ۹۰ دقیقه‌ای
```

---

## نصب روی سرور لینوکس (bare-metal)

با کاربر `root`:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/YOUR_ORG/dotinschool/main/scripts/install.sh)
```

> آدرس `YOUR_ORG/dotinschool` را با ریپوی واقعی خود جایگزین کنید (یا متغیر `DOTINSCHOOL_REPO_URL` را ست کنید).

اسکریپت به‌صورت خودکار: پیش‌نیازها و **Go** را نصب می‌کند، سورس را کلون می‌کند، باینری را می‌سازد، سرویس systemd می‌سازد (اجرای دائمی + ری‌استارت خودکار)، یک **رمز ادمین تصادفی** و یک **مسیر مخفی تصادفی** برای پنل مدیریت تولید می‌کند، و آدرس‌ها را چاپ می‌کند.

فرانت‌اند بیلدشده (`web/assets/`) داخل مخزن commit شده است، پس نصب سرور به Node نیاز ندارد.

### دستورات مدیریتی

| دستور | کاربرد |
|---|---|
| `dotinschool` | منوی مدیریت تعاملی (به سبک x-ui) |
| `dotinschool admin` | نمایش/تغییر آدرس مخفی، نام‌کاربری، رمز، پورت، ساب‌دامین |
| `dotinschool update` | دریافت آخرین نسخه از گیت‌هاب + کامپایل مجدد + ری‌استارت |
| `dotinschool update-menu` | به‌روزرسانی فقط همین اسکریپت |
| `dotinschool cert` | گرفتن/تمدید گواهی SSL از Let's Encrypt (acme.sh standalone) |
| `dotinschool start` / `stop` / `restart` / `status` | کنترل سرویس |
| `dotinschool enable` / `disable` | اجرای خودکار در بوت |
| `dotinschool uninstall` | حذف کامل |

یونیت systemd یک `ExecStartPre` دارد که در هر ری‌استارت باینری را از سورس فعلی می‌سازد؛ به همین دلیل دکمهٔ **«به‌روزرسانی»** داخل پنل (که `git reset --hard origin/main` + ری‌استارت می‌کند) واقعاً اعمال می‌شود. اگر بیلد شکست بخورد، باینری قبلیِ سالم دست‌نخورده می‌ماند.

### SSL

```bash
dotinschool cert
```

بدون دامنه (فقط IP)، این مرحله را رد کنید — پنل روی HTTP معمولی بدون خطای گواهی کار می‌کند.

---

## نصب با Docker

> **چرا این روش:** باینری روی **هاست** کامپایل می‌شود (Go بومی و سریع، فرانت از قبل در مخزن است)
> و `docker build` فقط یک کپیِ چندثانیه‌ایِ فایل داخل Alpine است. بنابراین بیلد Docker
> **هیچ‌وقت** به‌خاطر کمبود حافظهٔ Docker Desktop، بلاک‌بودن `proxy.golang.org`، یا قطع
> BuildKit (`rpc error: code = Unavailable ... EOF`) شکست نمی‌خورد.

### روش سریع (اسکریپت — روی سرور لینوکس)

```bash
bash <(curl -Ls https://raw.githubusercontent.com/YOUR_ORG/dotinschool/main/build/docker-install.sh)
```

Docker و Go را (در صورت نبود) نصب می‌کند، `.env` با رمز و مسیر مخفی تصادفی می‌سازد، ایمیج را
با کامپایل روی هاست می‌سازد، استک را بالا می‌آورد و دستور `dotinschool` را نصب می‌کند
(`up | down | restart | logs | ps | rebuild | update | admin`).

### دستی

```bash
cp .env.example .env      # ADMIN_PASSWORD و در صورت نیاز ADMIN_PATH را پر کنید
# روی ویندوز چون پورت ۸۰۸۰ را Docker Desktop می‌گیرد: APP_PORT=8091

make image                                    # = scripts/docker-build.sh
docker compose -f build/docker-compose.yml up -d
```

`make image` باینری `dist/dotinschool_linux_amd64` را می‌سازد و ایمیج `dotinschool:local` را
پکیج می‌کند. برای معماری‌های دیگر: `scripts/docker-build.sh "amd64 arm64"`.

- پنل: <http://localhost:8080/>  (روی ویندوز: پورتی که در `APP_PORT` گذاشتید · ادمین: `/<ADMIN_PATH>/`)
- داده‌ها در ولوم `./data` — SQLite تک‌فایل.

> بیلدِ کاملاً درون‌کانتینری (بدون نیاز به Go روی هاست) هم هست: `build/Dockerfile.hermetic` —
> ولی برای مرحلهٔ `go build` به حدود ۲ گیگابایت RAM آزاد برای Docker نیاز دارد، وگرنه همان
> خطای `rpc ... EOF` را می‌دهد. اگر دیدید، حافظهٔ Docker Desktop را در Settings → Resources
> بالا ببرید یا از `make image` استفاده کنید.

### HTTPS خودکار با دامنه (Caddy)

```bash
# در .env مقدار DOMAIN=panel.example.com را ست کنید (رکورد A به IP سرور اشاره کند)
docker compose -f build/docker-compose.yml -f build/docker-compose.prod.yml up -d
```

Caddy گواهی Let's Encrypt را خودش می‌گیرد و تمدید می‌کند — بدون certbot، بدون downtime.

داخل Docker، دکمهٔ «به‌روزرسانی» پنل پیام می‌دهد که روی هاست `dotinschool update` (یا `dotinschool rebuild`) را اجرا کنید — این دستور `git reset --hard origin/main`، سپس کامپایل مجدد باینری روی هاست و `docker compose up -d` را انجام می‌دهد. داده‌ها در ولوم `data/` دست‌نخورده می‌مانند.

---

## آرتیفکت‌های ریلیز (کراس‌کامپایل)

```bash
scripts/build.sh          # linux/amd64، linux/arm64، windows/amd64 → dist/
```

---

## پیکربندی

همهٔ مقادیر از متغیرهای محیطی یا فایل `.env` خوانده می‌شوند (نمونه: `.env.example`). مقداری که از قبل export شده باشد (systemd/Docker) بر `.env` اولویت دارد.

| متغیر | پیش‌فرض | توضیح |
|---|---|---|
| `PORT` | `8080` (یا `443` با TLS) | پورت گوش‌دادن |
| `DATA_DIR` | `<workdir>/data` | محل `arena.db` و `ai-config.json` |
| `ADMIN_USERNAME` | `administrator` | |
| `ADMIN_PASSWORD` | *تصادفی* | اگر خالی باشد ساخته و در `.env` ذخیره می‌شود |
| `ADMIN_PATH` | `admin` | مسیر مخفی پنل ادمین (بدون `/`) |
| `ADMIN_SUBDOMAIN` | — | ساب‌دامین اختصاصی ادمین (روتینگ بر اساس هدر Host) |
| `SSL_CERT_PATH` / `SSL_KEY_PATH` | — | اگر هر دو ست و موجود باشند، سرور مستقیم HTTPS می‌دهد |
| `IPPANEL_API_KEY` و پترن‌ها | — | پیامک OTP (اختیاری) — بدون کلید، ثبت‌نام/ورود کار نمی‌کند |
| `AI_API_KEY` / `AI_BASE_URL` / `AI_MODEL` | — | فقط fallback؛ اولویت با سرویس فعالِ پنل ← `data/ai-config.json` ← این متغیرها |
| `LOG_LEVEL` | `info` | `trace\|debug\|info\|warn\|error` |

---

## معماری

```
cmd/server/            بوت‌استرپ Gin + graceful shutdown
cmd/seed/              اسکریپت دادهٔ دمو
internal/config/       لود کانفیگ (env + .env + پیش‌فرض‌ها)
internal/database/     init GORM/SQLite، AutoMigrate، seed پیش‌فرض
internal/model/        Participant، Setting، ClientError، Challenge، AISettings
internal/service/      Participant، Setting، Session، RateLimiter، OTP،
                       SMSProvider (interface + ippanel)، AI (کلاینت OpenAI-compatible)، Scheduler
internal/apperr/       شکل واحد خطای HTTP (status + code + پیام فارسی)
internal/web/          روتر Gin، میدل‌ورها (security headers/CSP، admin auth،
                       admin-host)، هندلرهای /api، سرو استاتیکِ embed‌شده
web/                   سورس فرانت (Vue 3 + Vite + Ant Design Vue 4)
web/assets/            خروجی بیلد فرانت (commit می‌شود؛ go:embed)
scripts/               install.sh (bare-metal) · build.sh (کراس‌کامپایل) · docker-build.sh
build/                 Dockerfile (پکیجینگ) · Dockerfile.hermetic · docker-compose*.yml · Caddyfile · docker-install.sh
design-system/         خروجی اسکیل ui-ux-pro-max (MASTER.md + pages/admin.md)
```

### قرارداد API

پایدار و مطابق نسخهٔ Node.js. خلاصه:

| متد و مسیر | احراز | توضیح |
|---|---|---|
| `POST /api/otp/request` | عمومی، rate-limit | کد ۵رقمی، TTL ۲دقیقه، cooldown ۶۰ث |
| `POST /api/register` | عمومی، rate-limit | participant + `sessionToken` + SMS اطلاع به ادمین |
| `POST /api/lookup` | عمومی، rate-limit | ورود مجدد با موبایل + کد |
| `POST /api/session` | توکن نشست کاربر | تمدید نشست (sliding، ۳۰ روز) |
| `GET /api/config` | عمومی | وضعیت رویداد + چالش منتشرشده |
| `PUT /api/config` | ادمین | ذخیرهٔ فیلدهای رویداد/چالش (اعتبارسنجی dataURL تصویر، سقف ۶۰۰ کاراکتر پیام) |
| `GET /api/participants` | ادمین | فهرست |
| `PATCH /api/participants/:id` | ادمین **یا** توکن همان کاربر | نام و `aiScore` فقط ادمین؛ `started/submitted` فقط وقتی رویداد زنده است (قفل سمت سرور) |
| `DELETE /api/participants/:id` | ادمین | |
| `POST /api/admin/login` / `logout` | — / ادمین | مقایسهٔ timing-safe، توکن ۱۲ساعته |
| `GET /api/admin/backup` | ادمین | فایل SQLite با `VACUUM INTO` |
| `POST /api/admin/backup/restore` | ادمین | چک هدر SQLite، نگه‌داشتن نسخهٔ pre-restore، ری‌استارت graceful |
| `POST /api/admin/update` | ادمین | Docker: پیام راهنما؛ bare-metal: `git fetch/reset origin/main` + ری‌استارت |
| `GET/PUT /api/admin/ai-settings`، `.../test`، `.../ai-detect`، `.../ai-status` | ادمین | مدیریت سرویس‌های سازگار با OpenAI (کلید ماسک‌شده، تشخیص از روی امضا + پروب واقعی `GET /models`) |
| `POST /api/admin/review-code` | ادمین، rate-limit | بررسی کد جاوا؛ خروجی `{review, aiScore, model, usage}`؛ خط اولِ پاسخِ AI باید `AI_GENERATED_PERCENT: N` باشد |
| `POST /api/client-errors` | عمومی، rate-limit | ثبت خطای ارسال (هویت از توکن، سقف ۵۰۰ ردیف، هرگز hard-fail نمی‌کند) |
| `GET/seen/DELETE /api/admin/client-errors[...]` | ادمین | مشاهده/دیده‌شده/حذف |

---

## امنیت

- هدرهای امنیتی + CSP روی **همهٔ** پاسخ‌ها (script-src `'self'`؛ connect/img/default به `'self'` محدود).
- مسیر مخفی و تصادفی پنل ادمین + امکان ساب‌دامین اختصاصی (روتینگ بر اساس Host).
- rate-limit حافظه‌ای روی مسیرهای حساس؛ مقایسهٔ timing-safe برای رمز و OTP.
- TTL نشست: ادمین ۱۲ ساعت، کاربر ۳۰ روز (sliding). OTP: ۲دقیقه TTL، ۶۰ث cooldown، ۵ تلاش.
- هیچ جزئیات داخلی (مسیر فایل، stack trace) به کلاینت درز نمی‌کند.
- مهاجرت سبک خودکار (AutoMigrate) برای دیتابیس‌های قدیمی‌تر.
- ری‌استارت graceful برای بکاپ‌ریستور و آپدیت (systemd/Docker دوباره بالا می‌آورد).

---

## تفاوت‌های عمدی با نسخهٔ Node.js

- **نشست ادمین** با توکن Bearer در حافظهٔ سرور نگه داشته می‌شود (همان قرارداد API نسخهٔ قبل). وابستگی `gin-contrib/sessions` در این نسخه استفاده نشده تا قرارداد API دست‌نخورده بماند.
- **رندر PHP** حذف شده؛ به‌جای دو فایل HTML مجزا، یک SPA واحد است که سرور با تزریق `<meta name="dotin-mode">` (سازگار با CSP سخت‌گیرانه) مشخص می‌کند پنل کاربر است یا ادمین.
- بررسی هدر بکاپ نسبت به نسخهٔ قبل اصلاح شده تا فایل‌های واقعی SQLite را رد نکند (بررسی `SQLite format 3`).
