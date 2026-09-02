import type { MessageTree } from './index'

// Persian — the source of truth. Add every new user-facing string here.
export const fa: MessageTree = {
  app: {
    userTitle: 'چالش برنامه‌نویسی',
    adminTitle: 'پنل مدیریت',
  },
  nav: {
    overview: 'نمای کلی رویداد',
    schedule: 'زمان‌بندی',
    challenge: 'طراحی چالش',
    participants: 'شرکت‌کنندگان',
    ai: 'بررسی هوشمند',
    assistant: 'دستیار کد',
    errors: 'خطاهای کاربران',
    logout: 'خروج',
  },
  auth: {
    register: 'ثبت‌نام',
    login: 'ورود',
    firstName: 'نام',
    lastName: 'نام خانوادگی',
    phone: 'شماره موبایل',
    otp: 'کد تأیید پیامک‌شده',
    sendOtp: 'ارسال کد تأیید پیامکی',
    resendIn: 'ارسال مجدد ({sec})',
    submitRegister: 'ثبت‌نام و مشاهده چالش',
    submitLogin: 'ورود',
    rememberNote: 'نشست شما روی این مرورگر به‌خاطر سپرده می‌شود؛ دفعهٔ بعد بدون کد وارد می‌شوید.',
  },
  event: {
    waiting: 'رویداد زمان‌بندی شده و هنوز شروع نشده است',
    paused: 'رویداد توسط مدیر موقتاً متوقف شده است',
    ended: 'زمان رویداد به پایان رسیده است',
    remaining: 'زمان باقی‌مانده',
    untilStart: 'تا شروع خودکار',
    untilEnd: 'تا پایان رویداد',
  },
  challenge: {
    start: 'شروع حل چالش',
    submit: 'ارسال کد',
    resubmit: 'ارسال مجدد',
    retry: 'تلاش مجدد',
    sampleIn: 'ورودی نمونه',
    sampleOut: 'خروجی نمونه',
    scoreByCorrectness: 'امتیازدهی بر اساس درستی خروجی',
    scoreBySpeed: 'امتیازدهی بر اساس سرعت اجرا — هدف: {ms} میلی‌ثانیه',
  },
  common: {
    save: 'ذخیره',
    cancel: 'انصراف',
    delete: 'حذف',
    edit: 'ویرایش',
    refresh: 'به‌روزرسانی',
    close: 'بستن',
    loading: 'در حال بارگذاری…',
  },
}
