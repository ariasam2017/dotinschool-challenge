// Quiz game API — the player loop (token in body) and the admin board/bank.

import { apiFetch } from './client'

export type GamePhase = 'ready' | 'question' | 'reveal' | 'finished'
export type GameStatus = 'registered' | 'playing' | 'finished'

export type Difficulty = 'easy' | 'hard'

export interface GameQuestionView {
  index: number
  category: string
  difficulty: Difficulty
  text: string
  options: [string, string, string, string]
  limitMs: number
  remainingMs: number
}

export interface GameReveal {
  index: number
  choice: number // -1 = time ran out
  correct: number
  isCorrect: boolean
  timedOut: boolean
}

export interface GameResult {
  correct: number
  total: number
  finishedAt: number
  finishMessage: string
}

export interface GameReviewItem {
  category: string
  difficulty?: Difficulty
  text: string
  options: [string, string, string, string]
  choice: number // -1 timeout, -2 not answered yet (admin live view)
  correct: number
  isCorrect: boolean
  ms?: number // admin review only
}

export interface GameState {
  player: { id: string; firstName: string; lastName: string; phone: string; status: GameStatus }
  phase: GamePhase
  open: boolean
  testMode: boolean
  questionsPerGame: number
  secondsPerQuestion: number
  total: number
  index: number
  progress: ('correct' | 'wrong' | 'pending')[]
  question: GameQuestionView | null
  reveal: GameReveal | null
  result: GameResult | null
  review: GameReviewItem[] | null
}

export interface GameInfo {
  open: boolean
  questionsPerGame: number
  secondsPerQuestion: number
}

export interface GameSettings {
  open: boolean
  questionsPerGame: number
  secondsPerQuestion: number
  finishMessage: string
  /** winner-reveal animation, seconds */
  revealThirdSec: number
  revealSecondSec: number
  revealFirstSec: number
  revealGapSec: number
  /** players may replay from the result screen */
  testMode: boolean
}

export interface BoardRow {
  rank: number
  id: string
  firstName: string
  lastName: string
  phone: string
  status: GameStatus
  createdAt: number
  startedAt: number
  finishedAt: number
  correct: number
  answered: number
  total: number
  totalMs: number
  score: number
}

export interface GameBoard {
  settings: GameSettings
  questionCount: number
  easyCount: number
  hardCount: number
  /** finished players played different rounds (count/time changed mid-competition) */
  mixedRounds: boolean
  players: BoardRow[]
  podium: BoardRow[]
  stats: {
    registered: number
    playing: number
    finished: number
    total: number
    avgScore: number
    avgCorrect: number
  }
  serverNow: number
}

export interface GameQuestion {
  id: string
  category: string
  difficulty: Difficulty
  text: string
  options: [string, string, string, string]
  correct: number
  createdAt: number
}

export type GameQuestionInput = Omit<GameQuestion, 'id' | 'createdAt'>

/* ---- player ---- */
export const gameInfo = () => apiFetch<GameInfo>('/api/game/info')
export const gameRegister = (firstName: string, lastName: string, phone: string) =>
  apiFetch<{ token: string; created: boolean; state: GameState }>('/api/game/register', {
    method: 'POST',
    body: { firstName, lastName, phone },
  })
export const gameState = (token: string) =>
  apiFetch<GameState>('/api/game/state', { method: 'POST', body: { token } })
export const gameStart = (token: string) =>
  apiFetch<GameState>('/api/game/start', { method: 'POST', body: { token } })
export const gameAnswer = (token: string, index: number, choice: number) =>
  apiFetch<GameState>('/api/game/answer', { method: 'POST', body: { token, index, choice } })
export const gameReplay = (token: string) =>
  apiFetch<GameState>('/api/game/replay', { method: 'POST', body: { token } })
export const gameNext = (token: string) =>
  apiFetch<GameState>('/api/game/next', { method: 'POST', body: { token } })

/* ---- admin ---- */
export const gameBoard = () => apiFetch<GameBoard>('/api/admin/game/board')
export const saveGameSettings = (s: GameSettings) =>
  apiFetch<GameSettings>('/api/admin/game/settings', { method: 'PUT', body: s })
export const gamePlayerReview = (id: string) =>
  apiFetch<{ review: GameReviewItem[] }>('/api/admin/game/players/' + encodeURIComponent(id))
export const resetGamePlayer = (id: string) =>
  apiFetch<{ ok: true }>('/api/admin/game/players/' + encodeURIComponent(id) + '/reset', {
    method: 'POST',
  })
export const deleteGamePlayer = (id: string) =>
  apiFetch<{ ok: true }>('/api/admin/game/players/' + encodeURIComponent(id), { method: 'DELETE' })
export const deleteAllGamePlayers = () =>
  apiFetch<{ ok: true; deleted: number }>('/api/admin/game/players', { method: 'DELETE' })

export const listGameQuestions = () => apiFetch<GameQuestion[]>('/api/admin/game/questions')
export const createGameQuestion = (q: GameQuestionInput) =>
  apiFetch<GameQuestion>('/api/admin/game/questions', { method: 'POST', body: q })
export const updateGameQuestion = (id: string, q: GameQuestionInput) =>
  apiFetch<GameQuestion>('/api/admin/game/questions/' + encodeURIComponent(id), {
    method: 'PUT',
    body: q,
  })
export const deleteGameQuestions = (ids: string[]) =>
  apiFetch<{ ok: true; deleted: number }>('/api/admin/game/questions/delete', {
    method: 'POST',
    body: { ids },
  })
export const importGameQuestions = (questions: GameQuestionInput[], replace: boolean) =>
  apiFetch<{ ok: true; added: number; skipped: number }>('/api/admin/game/questions/import', {
    method: 'POST',
    body: { questions, replace },
  })
export const addDefaultGameQuestions = () =>
  apiFetch<{ ok: true; added: number }>('/api/admin/game/questions/defaults', { method: 'POST' })

/** The easy/hard order of an n-question game (mirrors model.DifficultyPlan). */
export function difficultyPlan(n: number): Difficulty[] {
  const ten: Difficulty[] = ['easy', 'easy', 'easy', 'hard', 'hard', 'easy', 'easy', 'easy', 'easy', 'hard']
  const five: Difficulty[] = ['easy', 'easy', 'easy', 'hard', 'hard']
  const tails: Difficulty[][] = [[], ['easy'], ['easy', 'hard'], ['easy', 'easy', 'hard'], ['easy', 'easy', 'easy', 'hard']]
  const plan: Difficulty[] = []
  while (n - plan.length >= 10) plan.push(...ten)
  if (n - plan.length >= 5) plan.push(...five)
  plan.push(...tails[n - plan.length])
  return plan
}
export const DIFFICULTY_LABEL: Record<Difficulty, string> = { easy: 'آسان', hard: 'سخت' }

/* ---- formatting ---- */
/** "۱۲٫۴ ثانیه" style seconds from ms, one decimal. */
export function fmtSecs(ms: number): string {
  const v = Math.round(ms / 100) / 10
  return v.toLocaleString('fa-IR', { maximumFractionDigits: 1 })
}
