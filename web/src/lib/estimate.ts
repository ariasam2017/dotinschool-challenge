// Client-side scoring estimate. There is no sandboxed JVM on the server, so
// "performance" scoring is a static estimate (loop-nesting depth + code size),
// clearly labelled as such in the admin panel — never a real run. Ported from
// app.js so the numbers the admin sees are unchanged.

export interface ExecEstimate {
  ms: number
  maxDepth: number
  loopCount: number
}

export function estimateExecution(code: string): ExecEstimate {
  const lines = code.split('\n')
  let depth = 0
  let maxDepth = 0
  let loopCount = 0
  for (const raw of lines) {
    const line = raw.replace(/\/\/.*$/, '')
    if (/\b(for|while)\s*\(/.test(line)) {
      loopCount++
      depth++
      maxDepth = Math.max(maxDepth, depth)
    }
    const opens = (line.match(/\{/g) || []).length
    const closes = (line.match(/\}/g) || []).length
    if (closes > opens) depth = Math.max(0, depth - (closes - opens))
  }
  const jitter = 0.85 + Math.random() * 0.35
  const ms = Math.round((6 + Math.pow(9, maxDepth) + lines.length * 0.35 + loopCount * 3) * jitter)
  return { ms, maxDepth, loopCount }
}

export function performanceScore(ms: number, targetMs: number): number {
  const ratio = ms / targetMs
  let score: number
  if (ratio <= 1) score = 90 + Math.round(Math.random() * 10)
  else if (ratio <= 2) score = 55 + Math.round((2 - ratio) * 35)
  else if (ratio <= 4) score = 25 + Math.round(((4 - ratio) / 2) * 30)
  else score = Math.max(5, 25 - Math.round(ratio))
  return Math.min(100, Math.max(0, score))
}

/** Count only "error" severity issues, mirroring the editor's lint(). */
export function countErrors(code: string): number {
  // Cheap re-derivation: unbalanced brackets + unterminated strings.
  const stack: string[] = []
  const pairs: Record<string, string> = { ')': '(', ']': '[', '}': '{' }
  const opens = new Set(['(', '[', '{'])
  let errors = 0
  for (const raw of code.split('\n')) {
    const line = raw.replace(/\/\/.*$/, '')
    let inStr: string | null = null
    for (let i = 0; i < line.length; i++) {
      const ch = line[i]
      if (inStr) {
        if (ch === '\\') {
          i++
          continue
        }
        if (ch === inStr) inStr = null
        continue
      }
      if (ch === '"' || ch === "'") {
        inStr = ch
        continue
      }
      if (opens.has(ch)) stack.push(ch)
      else if (ch in pairs) {
        if (!stack.length || stack[stack.length - 1] !== pairs[ch]) errors++
        else stack.pop()
      }
    }
    if (inStr) errors++
  }
  return errors + stack.length
}

export const STARTER_CODE = `import java.util.*;

public class Main {
    public int[] twoSum(int[] nums, int target) {
        // کد شما اینجا

    }
}
`
