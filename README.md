# dotinschool

<div dir="rtl" align="right">

سامانهٔ آنلاین چالش برنامه‌نویسی، با معماری‌ای مشابه پنل‌های **x-ui / 3x-ui**: یک باینری استاتیک تک‌فایل به زبان **Go** که هم فرانت‌اند Vue بیلدشده را داخل خودش embed می‌کند و هم یک REST API کوچک روی SQLite دارد. دو پنل مجزا دارد: **پنل شرکت‌کننده** و **پنل مدیریت**.

روی **IP خام و HTTP ساده** بدون هیچ خطا کار می‌کند؛ اگر مسیر cert/key ست شده باشد مستقیم TLS می‌دهد.

## امکانات

- بک‌اند: Go 1.22+‏ · Gin · GORM + SQLite (درایور pure-Go، بدون CGO) · robfig/cron · logrus · `go:embed`
- فرانت‌اند: Vue 3 + Ant Design Vue 4 + Vite · RTL کامل · فارسی · ارقام فارسی · تقویم جلالی · فونت IRANYekanX + JetBrains Mono
- ادیتور کد: CodeMirror 6 (های‌لایت جاوا، شماره خط، اتوکامپلیت، تطبیق پرانتز، Find/Replace، لینت سمت کلاینت)
- استقرار: `scripts/install.sh` + systemd + دستور سراسری `dotinschool` · Docker (Dockerfile چندمرحله‌ای + compose + override پروداکشن با Caddy برای HTTPS خودکار)

---

## نصب سریع

هر سه دستور، همون لحظه که اجرا می‌کنید، توکن گیت‌هاب رو به‌صورت مخفی (مثل رمز عبور، دیده نمی‌شود) ازتان می‌پرسند — چیزی داخل خودِ دستور جای‌گذاری نمی‌کنید. بعد خودشان آخرین نسخه را می‌گیرند، پیش‌نیازهای لازم را نصب می‌کنند، و برنامه را بالا می‌آورند. همان دستور را دوباره اجرا کنید تا به‌روزرسانی بگیرید — این‌بار دیگر توکن نمی‌خواهد، چون همان اولی داخل remote کلون ذخیره شده.

مخزن **Private** است، پس فقط برای همین اولین اجرا یک توکن گیت‌هاب لازم دارید:

1. به [github.com/settings/personal-access-tokens/new](https://github.com/settings/personal-access-tokens/new) بروید.
2. Resource owner: `ariasam2017` · Repository access: Only select repositories → `dotinschool`.
3. Repository permissions → Contents: **Read-only**.
4. Generate token و مقدارش را کپی کنید (فقط همان لحظه نشان داده می‌شود).

**لینوکس، نصب دائمی به‌عنوان سرویس سیستم (بدون Docker):**

</div>

```bash
bash -c 'read -rsp "GitHub token: " T; echo; DOTINSCHOOL_GH_TOKEN="$T" bash <(curl -Ls -H "Authorization: token $T" https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/install.sh)'
```

<div dir="rtl" align="right">

**لینوکس، اجرا روی Docker** (اگر Docker از قبل نصب نباشد، خودش نصبش می‌کند):

</div>

```bash
bash -c 'read -rsp "GitHub token: " T; echo; DOTINSCHOOL_GH_TOKEN="$T" bash <(curl -Ls -H "Authorization: token $T" https://raw.githubusercontent.com/ariasam2017/dotinschool/main/build/docker-install.sh)'
```

<div dir="rtl" align="right">

**ویندوز، اجرای محلی روی Docker** (Go را اگر نبود خودش مستقیم از go.dev دانلود می‌کند؛ اما Docker Desktop را نه — اگر نصب نباشد فقط پیام می‌دهد که نصبش کنید و دوباره همین دستور را بزنید):

</div>

```powershell
$s = Read-Host "GitHub token" -AsSecureString; $T = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s)); $env:DOTINSCHOOL_GH_TOKEN = $T; irm -Headers @{Authorization="token $T"} https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/windows-install.ps1 | iex
```

<div dir="rtl" align="right">

اگر ترجیح می‌دهید توکن را مستقیم داخل خودِ دستور (بدون پرامپت تعاملی) بگذارید — مثلاً برای اسکریپت‌های خودکار — همان `git clone https://YOUR_TOKEN@github.com/ariasam2017/dotinschool.git` هم جواب می‌دهد؛ جزئیات هر دو روش در ادامهٔ همین سند آمده است.

---

## اجرای محلی (توسعه)

پیش‌نیاز: Go 1.24+‏ و Node 20+‏. مخزن Private است؛ اگر git شما از قبل به گیت‌هاب وصل نیست (مثلاً با `gh auth login`)، به‌جای آدرس ساده از `https://<TOKEN>@github.com/ariasam2017/dotinschool.git` با همان توکنِ بخش «نصب سریع» استفاده کنید.

</div>

```bash
git clone https://github.com/ariasam2017/dotinschool.git
cd dotinschool

# 1) build the frontend (output goes to web/assets/, embedded via go:embed)
cd web && npm install && npm run build && cd ..

# 2) run the server
go run ./cmd/server
```

<div dir="rtl" align="right">

- پنل کاربر: <http://127.0.0.1:8080/>
- پنل ادمین: <http://127.0.0.1:8080/admin/> (روی نصب لینوکسی، مسیر مخفی تصادفی)
- رمز ادمین: اگر `ADMIN_PASSWORD` در `.env` نباشد، یک رمز تصادفی ساخته، در `.env` ذخیره و در لاگ چاپ می‌شود.

### حالت توسعهٔ داغ (hot-reload فرانت)

</div>

```bash
make dev      # Go server on :8080 + Vite dev server on :5173, proxied through /api
```

<div dir="rtl" align="right">

### دادهٔ دمو

</div>

```bash
go run ./cmd/seed     # wipes and reseeds the DB with ~16 participants and a live 90-minute event
```

<div dir="rtl" align="right">

---

## نصب روی سرور لینوکس (bare-metal)

اسکریپت نصب همه‌چیز را خودش انجام می‌دهد — از نصب پیش‌نیازهای سیستم تا بالا آوردن سرویس؛ فقط با کاربر `root` اجرا کنید (توکن گیت‌هاب را همان‌جا تعاملی می‌پرسد، توضیحش در «نصب سریع» بالا):

</div>

```bash
bash -c 'read -rsp "GitHub token: " T; echo; DOTINSCHOOL_GH_TOKEN="$T" bash <(curl -Ls -H "Authorization: token $T" https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/install.sh)'
```

<div dir="rtl" align="right">

اسکریپت به‌صورت خودکار: پیش‌نیازهای سیستم (`curl`, `git`, `tar`, `openssl`) و **Go** را نصب می‌کند، سورس را کلون می‌کند، باینری را می‌سازد، سرویس systemd می‌سازد (اجرای دائمی + ری‌استارت خودکار)، یک **رمز ادمین تصادفی** و یک **مسیر مخفی تصادفی** برای پنل مدیریت تولید می‌کند، و در پایان آدرس‌ها و اطلاعات ورود را چاپ می‌کند. تمام پیام‌های اسکریپت انگلیسی است و منوی مدیریتش به سبک x-ui/Sanaei جعبه‌ای است.

فرانت‌اند بیلدشده (`web/assets/`) داخل مخزن commit شده است، پس نصب سرور به Node نیاز ندارد.

### دستورات مدیریتی

| دستور | کاربرد |
|---|---|
| `dotinschool` | منوی مدیریت تعاملی (به سبک x-ui) |
| `dotinschool admin` | نمایش/تغییر آدرس مخفی، نام‌کاربری، رمز، پورت، ساب‌دامین |
| `dotinschool update` | دریافت آخرین نسخه از گیت‌هاب + کامپایل مجدد + ری‌استارت |
| `dotinschool update-menu` | به‌روزرسانی فقط همین اسکریپت |
| `dotinschool cert` | گرفتن/تمدید گواهی SSL از Let's Encrypt، روش HTTP-01 (نیاز به پورت ۸۰ باز) |
| `dotinschool cert-cf` | گرفتن/تمدید گواهی SSL از راه Cloudflare DNS (بدون نیاز به پورت ۸۰) |
| `dotinschool log` | لاگ زنده یا پاک‌کردن لاگ‌ها |
| `dotinschool reset` | بازنشانی کامل به پیش‌فرض — پاک‌کردن دیتابیس، رمز و مسیر مخفیِ تازه (مخرب، تأیید می‌خواهد) |
| `dotinschool fail2ban` | فعال‌سازی بن خودکار IP بعد از چند بار ورود اشتباه به پنل ادمین |
| `dotinschool banlog` | وضعیت جیل fail2ban مخصوص dotinschool |
| `dotinschool warp` | رفع مشکل اتصال به گیت‌هاب با Cloudflare WARP (برخی شبکه‌ها گیت‌هاب را کامل بلاک می‌کنند) |
| `dotinschool start` / `stop` / `restart` / `status` | کنترل سرویس |
| `dotinschool enable` / `disable` | اجرای خودکار در بوت |
| `dotinschool uninstall` | حذف کامل |

موقع نصب، اگر دامنه‌ای داشته باشید که به IP این سرور اشاره می‌کند، خودِ اسکریپت در همان لحظه ازتان می‌پرسد و SSL کامل (HTTP-01 یا Cloudflare DNS) را همان‌جا برایتان فعال می‌کند — نیازی به اجرای دستیِ `dotinschool cert` بعد از نصب نیست، مگر بخواهید بعداً اضافه‌اش کنید.

### اگر `git clone` روی سرور گیر کرد و چیزی دانلود نشد

</div>

```bash
git clone https://YOUR_TOKEN@github.com/ariasam2017/dotinschool.git
```

<div dir="rtl" align="right">

بعضی شبکه‌ها (مثل خیلی از سرورهای ایران) گیت‌هاب را کامل روی IP بلاک می‌کنند — نه فقط DNS، خودِ اتصال TCP هم تایم‌اوت می‌شود. علامتش این است: بعد از «Cloning into...» هیچ پیشرفتی نمی‌بینید و بعد از چند دقیقه با `Connection timed out` شکست می‌خورد. راه‌حل، مسیرکردن ترافیک از راه Cloudflare WARP است (رایگان، رسمی):

</div>

```bash
curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor --output /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/cloudflare-client.list
apt-get update && apt-get install -y cloudflare-warp
warp-cli --accept-tos registration new
warp-cli --accept-tos connect
```

<div dir="rtl" align="right">

روی CentOS/RHEL/Fedora به‌جای سه خط اول:

</div>

```bash
curl -fsSL https://pkg.cloudflareclient.com/cloudflare-warp-ascii.repo -o /etc/yum.repos.d/cloudflare-warp.repo
dnf install -y cloudflare-warp   # or: yum install -y cloudflare-warp
```

<div dir="rtl" align="right">

با `warp-cli status` مطمئن شوید وصل است، بعد دوباره `git clone` را امتحان کنید. بعد از این‌که یک‌بار `dotinschool` نصب شد، همین کار را با یک دستور هم می‌شود انجام داد: `dotinschool warp`.

یونیت systemd یک `ExecStartPre` دارد که در هر ری‌استارت باینری را از سورس فعلی می‌سازد؛ به همین دلیل دکمهٔ «به‌روزرسانی» داخل پنل (که `git reset --hard origin/main` + ری‌استارت می‌کند) واقعاً اعمال می‌شود. اگر بیلد شکست بخورد، باینری قبلیِ سالم دست‌نخورده می‌ماند.

### SSL (اگر موقع نصب رد کردید)

</div>

```bash
dotinschool cert       # HTTP-01, needs port 80 open
dotinschool cert-cf    # Cloudflare DNS, no need for port 80
```

<div dir="rtl" align="right">

بدون دامنه (فقط IP)، این مرحله را رد کنید — پنل روی HTTP معمولی بدون خطای گواهی کار می‌کند.

---

## نصب با Docker

### روی ویندوز — یک دستور (توکن را می‌پرسد + اجرا)

همان دستور «نصب سریع» بالا: توکن را تعاملی می‌پرسد، سورس را از گیت‌هاب می‌گیرد، Go را اگر نصب نباشد مستقیم از go.dev دانلود می‌کند (بدون winget، بدون نصب‌کننده)، بعد باینری لینوکس را روی هاست کامپایل و ایمیج را می‌سازد و استک را بالا می‌آورد. Docker Desktop را خودش نصب نمی‌کند — اگر نصب نباشد فقط پیام می‌دهد که نصبش کنید و دوباره اجرا کنید:

</div>

```powershell
$s = Read-Host "GitHub token" -AsSecureString; $T = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s)); $env:DOTINSCHOOL_GH_TOKEN = $T; irm -Headers @{Authorization="token $T"} https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/windows-install.ps1 | iex
```

<div dir="rtl" align="right">

اگر سورس از قبل روی سیستم کلون شده، همان مسیر را با اجرای مستقیم `.\docker-up.ps1` از داخل پوشهٔ پروژه هم می‌شود تکرار کرد (بدون گرفتن نسخهٔ تازه از گیت‌هاب). اگر PowerShell با خطای «running scripts is disabled on this system» جواب داد (پیش‌فرضِ رایج ویندوز)، به‌جایش این را بزنید: `powershell -ExecutionPolicy Bypass -File .\docker-up.ps1`

اولین بار که `docker-up.ps1` (یا `windows-install.ps1`) کامل اجرا شود، دستور سراسری **`dotinschool`** هم نصب می‌شود — دقیقاً معادل نسخهٔ لینوکسی Docker (بدون نیاز به دسترسی ادمین، در هر پنجرهٔ PowerShell/cmd باز یا جدیدی کار می‌کند):

| دستور | کاربرد |
|---|---|
| `dotinschool` | منوی مدیریت تعاملی (به سبک x-ui) |
| `dotinschool up` / `down` / `restart` | شروع / توقف / ری‌استارت استک |
| `dotinschool logs` / `ps` | لاگ زنده / وضعیت کانتینر |
| `dotinschool rebuild` | کامپایل مجدد + ساخت ایمیج + بازسازی (بدون گرفتن نسخهٔ جدید از گیت‌هاب) |
| `dotinschool update` | گرفتن آخرین نسخه از گیت‌هاب + کامپایل مجدد + ری‌استارت |
| `dotinschool admin` | نمایش آدرس پنل + نام‌کاربری/رمز ادمین |

### روی لینوکس / مک، از داخل سورس کلون‌شده

</div>

```bash
./docker-up.sh
```

<div dir="rtl" align="right">

خروجی هر دو مسیر: آدرس پنل‌ها + نام‌کاربری و رمز ادمین. اجرای دوباره فقط rebuild و recreate می‌کند.

بعداً برای گرفتن نسخهٔ جدید از گیت‌هاب و ری‌استارت، به‌جای `docker-up.ps1` / `docker-up.sh` همین‌ها را از همان پوشه بزنید (اول `git fetch` + `reset --hard` می‌کنند، بعد دقیقاً همان مراحل بالا را):

</div>

```powershell
.\update.ps1              # ویندوز
```
```bash
./update.sh               # لینوکس / مک
```

<div dir="rtl" align="right">

### روی سرور لینوکس، بدون هیچ‌چیز از قبل نصب‌شده

توکن گیت‌هاب را همان‌جا تعاملی می‌پرسد (توضیحش در «نصب سریع» بالا):

</div>

```bash
bash -c 'read -rsp "GitHub token: " T; echo; DOTINSCHOOL_GH_TOKEN="$T" bash <(curl -Ls -H "Authorization: token $T" https://raw.githubusercontent.com/ariasam2017/dotinschool/main/build/docker-install.sh)'
```

<div dir="rtl" align="right">

Docker و Go را نصب می‌کند، `.env` تصادفی می‌سازد، ایمیج را با کامپایل روی هاست می‌سازد، استک را بالا می‌آورد و دستور `dotinschool` را نصب می‌کند (`up | down | restart | logs | ps | rebuild | update | admin`).

### از ریشهٔ مخزن با compose خام

اگر Docker از قبل روی سیستم هست، این مسیر هم جواب می‌دهد — یک ایمیج (فرانت + باینری Go داخل خودش) و یک کانتینر `dotinschool` می‌سازد، ابزار دیگری لازم نیست:

</div>

```bash
docker compose up -d --build
```

<div dir="rtl" align="right">

- پنل کاربر: <http://localhost:8080/> · پنل ادمین: `http://localhost:8080/<ADMIN_PATH>/`
- رمز ادمین: اگر `.env` نساخته باشید، یک رمز تصادفی ساخته و در لاگ چاپ می‌شود (`docker compose logs app`). برای رمز ثابت: `cp .env.example .env` و `ADMIN_PASSWORD` را پر کنید.
- داده‌ها در ولوم `./data` — SQLite تک‌فایل.
- لاگ: `docker compose logs -f app` · توقف: `docker compose down`
- به‌روزرسانی: `./update.sh` (لینوکس/مک) یا `.\update.ps1` (ویندوز) — نسخهٔ جدید را از گیت‌هاب می‌گیرد،
  باینری را روی هاست دوباره می‌سازد و استک را ری‌استارت می‌کند؛ صرفاً دوباره زدن
  `docker compose up -d --build` کافی نیست چون باینری از قبل باید روی هاست کامپایل شده باشد.

روی ویندوز، Docker Desktop معمولاً پورت ۸۰۸۰ را گرفته — پورت میزبان را عوض کنید:

</div>

```powershell
$env:APP_PORT = "8091"; docker compose up -d --build
# or add the line APP_PORT=8091 to the .env file next to docker-compose.yml
```

<div dir="rtl" align="right">

> مرحلهٔ `go build` داخل ایمیج کمی حافظه می‌خواهد (با `-p 1` و `GOMEMLIMIT` تنظیم شده تا روی Docker Desktop معمولی جا شود). اگر باز هم با `rpc error: code = Unavailable ... EOF` مُرد، یا حافظهٔ Docker Desktop را در Settings → Resources بالا ببرید (4GB کافی است)، یا از `docker-up.ps1` / `docker-up.sh` بالا استفاده کنید که اصلاً `go build` داخل کانتینر ندارد.

### HTTPS خودکار با دامنه (Caddy)

</div>

```bash
# set DOMAIN=panel.example.com in .env (its DNS A record must point at this server)
docker compose -f build/docker-compose.yml -f build/docker-compose.prod.yml up -d
```

<div dir="rtl" align="right">

Caddy گواهی Let's Encrypt را خودش می‌گیرد و تمدید می‌کند — بدون certbot، بدون downtime.

داخل Docker، دکمهٔ «به‌روزرسانی» پنل پیام می‌دهد که از پوشهٔ ریشهٔ پروژه روی همان سرور `.\update.ps1` (ویندوز) یا `./update.sh` (لینوکس/مک) را اجرا کنید — یا اگر با `docker-install.sh` نصب کرده‌اید، دستور `dotinschool update` را. هر سه یک کار را انجام می‌دهند: `git reset --hard origin/main`، سپس کامپایل مجدد باینری روی هاست و `docker compose up -d`. داده‌ها در ولوم `data/` دست‌نخورده می‌مانند.

---

## آرتیفکت‌های ریلیز (کراس‌کامپایل)

</div>

```bash
scripts/build.sh          # linux/amd64, linux/arm64, windows/amd64 -> dist/
```

<div dir="rtl" align="right">

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

</div>

```
cmd/server/              Gin bootstrap + graceful shutdown
cmd/seed/                demo data seed script
internal/config/         config loading (env + .env + defaults)
internal/database/       GORM/SQLite init, AutoMigrate, default seed
internal/model/          Participant, Setting, ClientError, Challenge, AISettings
internal/service/        Participant, Setting, Session, RateLimiter, OTP,
                          SMSProvider (interface + ippanel), AI (OpenAI-compatible client), Scheduler
internal/apperr/         unified HTTP error shape (status + code + Persian message)
internal/web/            Gin router, middleware (security headers/CSP, admin auth,
                          admin-host), /api handlers, embedded static file serving
web/                      frontend source (Vue 3 + Vite + Ant Design Vue 4)
web/assets/               built frontend output (committed; go:embed)
scripts/                  install.sh (bare-metal), build.sh (cross-compile), docker-build.sh
build/                    Dockerfile (packaging), Dockerfile.hermetic, docker-compose*.yml, Caddyfile, docker-install.sh
design-system/            ui-ux-pro-max skill output (MASTER.md + pages/admin.md)
```

<div dir="rtl" align="right">

فرانت یک SPA واحد است؛ سرور با تزریق `<meta name="dotin-mode">` (سازگار با CSP سخت‌گیرانه) مشخص می‌کند همان بیلد باید به‌عنوان پنل کاربر بارگذاری شود یا پنل ادمین.

### قرارداد API

خلاصهٔ endpointها:

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
- نشست ادمین با توکن Bearer در حافظهٔ سرور نگه داشته می‌شود؛ مقایسهٔ رمز و کد OTP هم timing-safe است.
- rate-limit حافظه‌ای روی مسیرهای حساس؛ هر ورود ناموفق به پنل ادمین با IP در لاگ ثبت می‌شود (`dotinschool fail2ban` برای بن خودکار بعد از چند بار خطا).
- TTL نشست: ادمین ۱۲ ساعت، کاربر ۳۰ روز (sliding). OTP: ۲دقیقه TTL، ۶۰ث cooldown، ۵ تلاش.
- هیچ جزئیات داخلی (مسیر فایل، stack trace) به کلاینت درز نمی‌کند.
- مهاجرت سبک خودکار (AutoMigrate) برای دیتابیس‌های قدیمی‌تر.
- ری‌استارت graceful برای بکاپ‌ریستور و آپدیت (systemd/Docker دوباره بالا می‌آورد).

</div>
