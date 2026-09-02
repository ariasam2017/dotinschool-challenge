// Persian digits, phone/time formatting, and search normalisation. Mirrors the
// helpers the original store.js exposed so behaviour is identical.

import { toJalali } from './jalali'

const FA_DIGITS = ['۰', '۱', '۲', '۳', '۴', '۵', '۶', '۷', '۸', '۹']

/** Replace ASCII digits with Persian digits. */
export function toFa(input: string | number): string {
  return String(input).replace(/\d/g, (d) => FA_DIGITS[+d])
}

/** Replace Persian (and Arabic-Indic) digits with ASCII. */
export function toEn(input: string): string {
  return input
    .replace(/[۰-۹]/g, (d) => String(FA_DIGITS.indexOf(d)))
    .replace(/[٠-٩]/g, (d) => String('٠١٢٣٤٥٦٧٨٩'.indexOf(d)))
}

export function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

/** "HH:MM:SS" in Persian digits from a millisecond duration. */
export function fmtRemain(ms: number): string {
  if (ms < 0) ms = 0
  const total = Math.floor(ms / 1000)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return `${toFa(pad2(h))}:${toFa(pad2(m))}:${toFa(pad2(s))}`
}

/** "+۹۸ ۹۱۲ ۳۴۵ ۶۷۸۹" from a bare "9XXXXXXXXX". */
export function fmtPhone(p: string): string {
  if (!/^9\d{9}$/.test(p)) return toFa(p)
  return `+۹۸ ${toFa(p.slice(0, 3))} ${toFa(p.slice(3, 6))} ${toFa(p.slice(6))}`
}

/** "HH:MM" local time from an epoch-ms timestamp. */
export function fmtTime(ts: number): string {
  const d = new Date(ts)
  return `${toFa(pad2(d.getHours()))}:${toFa(pad2(d.getMinutes()))}`
}

/** Jalali date like "۱۴۰۳/۰۵/۱۲" from an epoch-ms timestamp. */
export function fmtJalaliDate(ts: number): string {
  const d = new Date(ts)
  const j = toJalali(d.getFullYear(), d.getMonth() + 1, d.getDate())
  return `${toFa(j.jy)}/${toFa(pad2(j.jm))}/${toFa(pad2(j.jd))}`
}

/** Jalali date + time, e.g. "۱۴۰۳/۰۵/۱۲ — ۰۹:۳۰". */
export function fmtJalaliDateTime(ts: number): string {
  const d = new Date(ts)
  return `${fmtJalaliDate(ts)} — ${toFa(pad2(d.getHours()))}:${toFa(pad2(d.getMinutes()))}`
}

/** Digits only, ASCII — for matching a phone against a search box. */
export function digitsOnly(s: string): string {
  return toEn(s || '').replace(/\D/g, '')
}

const DIFF_LABELS: Record<string, string> = { easy: 'آسان', medium: 'متوسط', hard: 'دشوار' }
export function diffLabel(d: string): string {
  return DIFF_LABELS[d] || 'متوسط'
}
export function diffTone(d: string): 'success' | 'warning' | 'danger' {
  return d === 'easy' ? 'success' : d === 'hard' ? 'danger' : 'warning'
}

const STATUS_LABELS: Record<string, string> = {
  registered: 'ثبت‌نام‌شده',
  started: 'در حال حل',
  submitted: 'ارسال‌شده',
}
export function statusLabel(s: string): string {
  return STATUS_LABELS[s] || STATUS_LABELS.registered
}
export function statusTone(s: string): 'neutral' | 'info' | 'success' {
  return s === 'submitted' ? 'success' : s === 'started' ? 'info' : 'neutral'
}
