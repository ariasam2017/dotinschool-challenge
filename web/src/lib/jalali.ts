// Self-contained Jalali <-> Gregorian conversion — no external library. Ported
// verbatim from the original admin.js: the Jalali calendar break-year table
// (same reference as jalaali-js) plus the Fliegel & Van Flandern Julian-day
// algorithm. Verified against Nowruz 1403, the 1403 leap year, and round-trips.

function div(a: number, b: number): number {
  return Math.floor(a / b)
}
function mod(a: number, b: number): number {
  return a - Math.floor(a / b) * b
}

interface JalCal {
  leap: number
  gy: number
  march: number
}

function jalCal(jy: number): JalCal {
  const breaks = [
    -61, 9, 38, 199, 426, 686, 756, 818, 1111, 1181, 1210, 1635, 2060, 2097, 2192, 2262, 2324,
    2394, 2456, 3178,
  ]
  const gy = jy + 621
  let leapJ = -14
  let jp = breaks[0]
  let jump = 0
  for (let i = 1; i < breaks.length; i++) {
    const jm = breaks[i]
    jump = jm - jp
    if (jy < jm) break
    leapJ = leapJ + div(jump, 33) * 8 + div(mod(jump, 33), 4)
    jp = jm
  }
  let n = jy - jp
  leapJ = leapJ + div(n, 33) * 8 + div(mod(n, 33) + 3, 4)
  if (mod(jump, 33) === 4 && jump - n === 4) leapJ += 1
  const leapG = div(gy, 4) - div((div(gy, 100) + 1) * 3, 4) - 150
  const march = 20 + leapJ - leapG
  if (jump - n < 6) n = n - jump + div(jump + 4, 33) * 33
  let leap = mod(mod(n + 1, 33) - 1, 4)
  if (leap === -1) leap = 4
  return { leap, gy, march }
}

function g2d(gy: number, gm: number, gd: number): number {
  const a = div(14 - gm, 12)
  const y = gy + 4800 - a
  const m = gm + 12 * a - 3
  return (
    gd + div(153 * m + 2, 5) + 365 * y + div(y, 4) - div(y, 100) + div(y, 400) - 32045
  )
}

function d2g(jdn: number): { gy: number; gm: number; gd: number } {
  const a = jdn + 32044
  const b = div(4 * a + 3, 146097)
  const c = a - div(146097 * b, 4)
  const d = div(4 * c + 3, 1461)
  const e = c - div(1461 * d, 4)
  const m = div(5 * e + 2, 153)
  const gd = e - div(153 * m + 2, 5) + 1
  const gm = m + 3 - 12 * div(m, 10)
  const gy = 100 * b + d - 4800 + div(m, 10)
  return { gy, gm, gd }
}

export function toGregorian(jy: number, jm: number, jd: number): { gy: number; gm: number; gd: number } {
  const r = jalCal(jy)
  const jdn = g2d(r.gy, 3, r.march) + (jm - 1) * 31 - div(jm, 7) * (jm - 7) + jd - 1
  return d2g(jdn)
}

export function toJalali(gy: number, gm: number, gd: number): { jy: number; jm: number; jd: number } {
  const jdnGreg = g2d(gy, gm, gd)
  const gy2 = d2g(jdnGreg).gy
  let jy = gy2 - 621
  const r = jalCal(jy)
  const jdn1f = g2d(r.gy, 3, r.march)
  let k = jdnGreg - jdn1f
  let jm: number
  let jd: number
  if (k >= 0) {
    if (k <= 185) {
      jm = 1 + div(k, 31)
      jd = mod(k, 31) + 1
      return { jy, jm, jd }
    }
    k -= 186
  } else {
    jy -= 1
    k += 179
    if (r.leap === 1) k += 1
  }
  jm = 7 + div(k, 30)
  jd = mod(k, 30) + 1
  return { jy, jm, jd }
}

/** Days in a Jalali month (1-6 => 31, 7-11 => 30, 12 => 29/30). */
export function jalaliMonthLength(jy: number, jm: number): number {
  if (jm <= 6) return 31
  if (jm <= 11) return 30
  return jalCal(jy).leap === 0 ? 30 : 29
}

export const JALALI_MONTHS = [
  'فروردین',
  'اردیبهشت',
  'خرداد',
  'تیر',
  'مرداد',
  'شهریور',
  'مهر',
  'آبان',
  'آذر',
  'دی',
  'بهمن',
  'اسفند',
]

/** Build a Date from a Jalali date + time in local time. */
export function jalaliToDate(jy: number, jm: number, jd: number, hh: number, mm: number): Date {
  const g = toGregorian(jy, jm, jd)
  return new Date(g.gy, g.gm - 1, g.gd, hh, mm, 0, 0)
}
