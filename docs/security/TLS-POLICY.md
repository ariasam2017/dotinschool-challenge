<div dir="rtl">

# سیاست اجباری‌سازی TLS و دسترسی شبکه‌ای

بندهای ۱، ۴.۱، ۴.۲ و ۶ الزامات واحد معماری امن.

## حالت‌های استقرار

| حالت | TLS | وضعیت عملیاتی |
|---|---|---|
| Bare-metal با گواهی (`SSL_CERT_PATH`/`SSL_KEY_PATH`) | برنامه مستقیم | ✅ مجاز |
| Cloudflare Tunnel (`dotinschool-challenge tunnel token`) | لبهٔ Cloudflare | ✅ مجاز (`TLS_PROXY=1`) |
| Docker + Caddy (`build/docker-compose.prod.yml` یا `.acme.yml`) | Caddy | ✅ مجاز |
| Docker با TLS مستقیم (`build/docker-compose.tls.yml`) | برنامه مستقیم | ✅ مجاز |
| Bare-metal بدون گواهی، `docker-compose.yml` پایه | ندارد | ❌ فقط توسعه/ارزیابی |

## روال استقرار: مرحلهٔ راه‌اندازی ← گواهی ← قفل HTTPS

هر دو منوی مدیریتی لینوکس (نصب مستقیم `dotinschool-challenge` و نصب Docker) یک روال دارند:

1. **مرحلهٔ راه‌اندازی (`APP_ENV=setup`)** — بلافاصله بعد از نصب. سامانه روی `http://<IP>:<پورت>` بالا می‌آید و ورود ادمین روی HTTP هم موقتاً مجاز است (`ALLOW_INSECURE_ADMIN=1`) تا نصب بررسی شود. این مرحله «قبل از بهره‌برداری» است و برای استفادهٔ واقعی مجاز نیست؛ در منو، لاگ و خروجی‌ها با برچسب SETUP نشان داده می‌شود.
2. **گرفتن گواهی** — از منوی SSL:
   - رایگان: Let's Encrypt (HTTP-01 یا Cloudflare DNS)، ZeroSSL؛ تمدید خودکار با acme.sh.
   - خریداری/سازمانی (Certum، CA شرکت، هر CA دیگر): مرحلهٔ ۱ ساخت کلید خصوصی و CSR **روی خود سرور** (کلید هیچ‌وقت سرور را ترک نمی‌کند)، مرحلهٔ ۲ نصب فایل‌هایی که CA می‌فرستد (PEM، DER ‎.cer/.crt، ‎.p7b، یا ‎.pfx/.p12 همراه کلید). پیش از نصب بررسی می‌شود: تطابق کلید و گواهی، منقضی نبودن، صدور برای همان دامنه، وجود گواهی میانی (ترتیب زنجیره خودکار درست می‌شود). این گواهی‌ها خودکار تمدید نمی‌شوند و تاریخ انقضا در وضعیت نمایش داده می‌شود.
3. **قفل HTTPS (`lock on`)** — پس از بررسی `https://` در مرورگر. `APP_ENV=production` می‌شود، `ALLOW_INSECURE_ADMIN` حذف می‌شود و شبکهٔ مجاز پنل مدیریت (`ADMIN_ALLOWED_CIDRS`) پرسیده می‌شود. اگر سامانه در حالت عملیاتی بالا نیاید، تغییر خودکار برمی‌گردد. از این لحظه هیچ مسیر HTTP ساده‌ای وجود ندارد؛ در Docker پورت برنامه هم فقط روی loopback منتشر می‌شود و «حذف HTTPS» تا وقتی قفل روشن است رد می‌شود.

خاموش کردن قفل (`lock off`) فقط با تایپ عبارت `UNLOCK` ممکن است. روشن و خاموش شدن قفل با نام کاربر در syslog ثبت می‌شود (`logger -t dotinschool-challenge`).

## سازوکار اجبار

با `APP_ENV=production` برنامه در شرایط زیر **اصلاً اجرا نمی‌شود** (پیام خطا در لاگ):

- نه گواهی مستقیم دارد و نه `TLS_PROXY=1`؛
- مسیر گواهی تنظیم شده ولی فایل وجود ندارد (بازگشت بی‌صدا به HTTP حذف شد)؛
- `ALLOW_INSECURE_ADMIN` روشن است؛
- `ADMIN_ALLOWED_CIDRS` خالی است یا ورودی نامعتبر دارد.

در حال اجرا، هر درخواستی که از کانال امن نیامده باشد با `403 TLS_REQUIRED` رد می‌شود. «کانال امن» یعنی:

1. TLS روی خود برنامه؛ یا
2. درخواست از پروکسی معتبر (`127.0.0.1`/`::1` یا `TRUSTED_PROXIES`) با `X-Forwarded-Proto: https`؛ یا
3. اتصال loopback بدون هیچ هدر پروکسی (healthcheck کانتینر، SSH tunnel) — این اتصال از شبکه عبور نکرده است.

هدر `X-Forwarded-Proto` از هر مبدأ دیگری نادیده گرفته می‌شود، پس قابل جعل نیست.

در فایل‌های compose دارای TLS (`prod`، `acme`، `tls`) مقدار پیش‌فرض `APP_ENV` برابر `production` و پورت برنامه روی `127.0.0.1` است؛ فقط `.env` مرحلهٔ راه‌اندازی (`APP_ENV=setup`، `APP_BIND=0.0.0.0`) آن را باز نگه می‌دارد، و `lock on` همین دو را به حالت قفل برمی‌گرداند.

## پنل مدیریت

- ورود و تمام APIهای اداری **در همهٔ حالت‌ها** (حتی توسعه) فقط روی کانال امن پاسخ می‌دهند. برای ارزیابی محلی روی HTTP ساده: `ALLOW_INSECURE_ADMIN=1` (در production ممنوع).
- `ADMIN_ALLOWED_CIDRS`: فهرست IP/CIDR شبکهٔ مدیریت (مثلاً `10.10.0.0/16,192.168.5.7`). خارج از آن، پنل، ورود و APIها پاسخ `404` می‌دهند و رخداد با IP ثبت می‌شود.
- مقدار `any` یعنی محدودیت عمداً به لبه سپرده شده (Cloudflare Access یا VPN) — باید در مستندات استقرار ثبت شود.
- مسیر مخفی پنل (`ADMIN_PATH`) کنترل امنیتی محسوب نمی‌شود؛ کنترل واقعی همین محدودیت شبکه‌ای و TLS است.

## پروتکل و رمزها

- حداقل TLS 1.2. برای 1.2 فقط: `ECDHE-{ECDSA,RSA}-AES{128,256}-GCM` و `ECDHE-{ECDSA,RSA}-CHACHA20-POLY1305`. مجموعه‌های TLS 1.3 همگی AEAD هستند.
- همین سیاست در `cmd/server/main.go` و هر سه Caddyfile اعمال شده است.
- HSTS: `max-age=31536000; includeSubDomains` در هر پاسخی که از کانال TLS آمده (برنامه و Caddy).

## IP واقعی بازدیدکننده

- فقط **یک** هدر معتبر است: `CLIENT_IP_HEADER` (پیش‌فرض `X-Forwarded-For`؛ پشت Cloudflare Tunnel: `CF-Connecting-IP` که نصب‌کننده خودکار تنظیم می‌کند).
- این هدر فقط از loopback یا `TRUSTED_PROXIES` پذیرفته می‌شود.
- Rate Limit، لاگ fail2ban و لاگ دسترسی همگی از همین یک مقدار استفاده می‌کنند.
- در Docker، Caddy و برنامه روی شبکهٔ اختصاصی `edge` با subnet ثابت (`172.30.57.0/24`) و IP ثابت هستند؛ فقط IP خود Caddy معتبر است (`EDGE_SUBNET`/`EDGE_CADDY_IP`/`EDGE_APP_IP` برای تغییر در صورت تداخل).

## نمونهٔ `.env` عملیاتی (Bare-metal)

</div>

```env
APP_ENV=production
SSL_CERT_PATH=/etc/dotinschool-challenge/ssl/panel.example.com/fullchain.pem
SSL_KEY_PATH=/etc/dotinschool-challenge/ssl/panel.example.com/privkey.pem
ADMIN_ALLOWED_CIDRS=10.10.0.0/16
```

<div dir="rtl">

پشت Cloudflare Tunnel به‌جای گواهی:

</div>

```env
APP_ENV=production
TLS_PROXY=1
CLIENT_IP_HEADER=CF-Connecting-IP
ADMIN_ALLOWED_CIDRS=any
```

<div dir="rtl">

(در حالت دوم محدودیت شبکه‌ای پنل باید با یک Cloudflare Access Application روی مسیر پنل اعمال شود.)

</div>
