// Synthesized sound effects for the winner-reveal show — heartbeat, roller
// ticks, a tension riser, and the reveal hit + fanfare. Everything is built
// with the Web Audio API, so there are no audio files to ship or load (and
// nothing for the strict CSP to block). An AudioContext may only start after
// a user gesture, so create the engine from a click handler.

export interface RevealSound {
  /** Two-thump "lub-dub"; louder = stronger beat. */
  heartbeat(intensity?: number): void
  /** A short click for each name the roller flips past. */
  tick(): void
  /** A rising whoosh over `seconds` — call once per roll. Returns a stop fn. */
  riser(seconds: number): () => void
  /** The reveal: drum hit + cymbal + fanfare; `big` for first place. */
  reveal(big: boolean): void
  setMuted(m: boolean): void
  close(): void
}

export function createRevealSound(): RevealSound | null {
  const AC: typeof AudioContext | undefined =
    window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!AC) return null
  const ctx = new AC()
  void ctx.resume()

  const master = ctx.createGain()
  master.gain.value = 1
  // the compressor stops stacked effects clipping while keeping them loud
  const comp = ctx.createDynamicsCompressor()
  comp.threshold.value = -14
  comp.ratio.value = 4
  comp.attack.value = 0.003
  comp.release.value = 0.25
  master.connect(comp).connect(ctx.destination)

  let noiseBuf: AudioBuffer | null = null
  function noise(): AudioBuffer {
    if (noiseBuf) return noiseBuf
    const len = ctx.sampleRate * 2
    noiseBuf = ctx.createBuffer(1, len, ctx.sampleRate)
    const d = noiseBuf.getChannelData(0)
    for (let i = 0; i < len; i++) d[i] = Math.random() * 2 - 1
    return noiseBuf
  }

  function thump(t: number, gain: number) {
    const o = ctx.createOscillator()
    const g = ctx.createGain()
    o.type = 'sine'
    o.frequency.setValueAtTime(110, t)
    o.frequency.exponentialRampToValueAtTime(38, t + 0.16)
    g.gain.setValueAtTime(0.0001, t)
    g.gain.exponentialRampToValueAtTime(gain, t + 0.012)
    g.gain.exponentialRampToValueAtTime(0.0001, t + 0.28)
    o.connect(g).connect(master)
    o.start(t)
    o.stop(t + 0.3)
  }

  /** Kick drum + sub-bass boom. */
  function boom(t: number, gain: number) {
    thump(t, gain)
    const o = ctx.createOscillator()
    const g = ctx.createGain()
    o.type = 'sine'
    o.frequency.setValueAtTime(70, t)
    o.frequency.exponentialRampToValueAtTime(32, t + 0.9)
    g.gain.setValueAtTime(0.0001, t)
    g.gain.exponentialRampToValueAtTime(gain * 0.8, t + 0.02)
    g.gain.exponentialRampToValueAtTime(0.0001, t + 1.1)
    o.connect(g).connect(master)
    o.start(t)
    o.stop(t + 1.2)
  }

  /** A bright, brassy note: detuned saw + square through an opening filter. */
  function brass(freq: number, t: number, dur: number, gain: number) {
    const g = ctx.createGain()
    const f = ctx.createBiquadFilter()
    f.type = 'lowpass'
    f.Q.value = 2
    f.frequency.setValueAtTime(900, t)
    f.frequency.exponentialRampToValueAtTime(6000, t + 0.08)
    f.frequency.exponentialRampToValueAtTime(2500, t + dur)
    g.gain.setValueAtTime(0.0001, t)
    g.gain.exponentialRampToValueAtTime(gain, t + 0.03)
    g.gain.setValueAtTime(gain * 0.85, t + dur * 0.7)
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur)
    f.connect(g).connect(master)
    for (const [type, detune] of [['sawtooth', -6], ['square', 6]] as const) {
      const o = ctx.createOscillator()
      o.type = type
      o.frequency.setValueAtTime(freq, t)
      o.detune.value = detune
      o.connect(f)
      o.start(t)
      o.stop(t + dur + 0.05)
    }
  }

  function cymbal(t: number, dur: number, gain: number) {
    const src = ctx.createBufferSource()
    src.buffer = noise()
    const hp = ctx.createBiquadFilter()
    hp.type = 'highpass'
    hp.frequency.value = 5000
    const g = ctx.createGain()
    g.gain.setValueAtTime(gain, t)
    g.gain.exponentialRampToValueAtTime(0.0001, t + dur)
    src.connect(hp).connect(g).connect(master)
    src.start(t)
    src.stop(t + dur)
  }

  return {
    heartbeat(intensity = 1) {
      const t = ctx.currentTime + 0.01
      const k = Math.min(1.4, Math.max(0.4, intensity))
      thump(t, 0.85 * k)
      thump(t + 0.19, 0.55 * k)
    },
    tick() {
      const t = ctx.currentTime
      const o = ctx.createOscillator()
      const g = ctx.createGain()
      o.type = 'square'
      o.frequency.setValueAtTime(1800, t)
      g.gain.setValueAtTime(0.035, t)
      g.gain.exponentialRampToValueAtTime(0.0001, t + 0.03)
      o.connect(g).connect(master)
      o.start(t)
      o.stop(t + 0.04)
    },
    riser(seconds) {
      const t = ctx.currentTime
      const end = t + Math.max(0.5, seconds)
      const o = ctx.createOscillator()
      const g = ctx.createGain()
      const f = ctx.createBiquadFilter()
      o.type = 'sawtooth'
      o.frequency.setValueAtTime(80, t)
      o.frequency.exponentialRampToValueAtTime(420, end)
      f.type = 'lowpass'
      f.frequency.setValueAtTime(300, t)
      f.frequency.exponentialRampToValueAtTime(2600, end)
      g.gain.setValueAtTime(0.0001, t)
      g.gain.exponentialRampToValueAtTime(0.06, end)
      o.connect(f).connect(g).connect(master)
      o.start(t)
      o.stop(end + 0.05)
      return () => {
        const n = ctx.currentTime
        g.gain.cancelScheduledValues(n)
        g.gain.setValueAtTime(g.gain.value, n)
        g.gain.exponentialRampToValueAtTime(0.0001, n + 0.08)
        try {
          o.stop(n + 0.1)
        } catch {
          /* already stopped */
        }
      }
    },
    reveal(big) {
      const t = ctx.currentTime + 0.02
      // a deep, punchy impact: kick + sub boom + bright crash
      boom(t, big ? 1.6 : 1.3)
      cymbal(t, big ? 3 : 2, big ? 0.5 : 0.38)
      // brass-like fanfare: two layered oscillators per note, bright filter
      const notes = big ? [523.25, 659.25, 783.99, 1046.5] : [523.25, 659.25, 783.99]
      notes.forEach((n, i) => brass(n, t + 0.14 + i * 0.14, 0.3, 0.32))
      const hold = t + 0.14 + notes.length * 0.14
      // the held chord — C major, full voicing for first place
      const chord = big ? [261.63, 523.25, 659.25, 783.99, 1046.5] : [523.25, 659.25, 783.99]
      chord.forEach((n) => brass(n, hold, big ? 2.4 : 1.4, big ? 0.22 : 0.2))
      if (big) {
        boom(hold, 1.4)
        cymbal(hold, 2.8, 0.42)
      }
    },
    setMuted(m) {
      master.gain.setTargetAtTime(m ? 0 : 1, ctx.currentTime, 0.02)
    },
    close() {
      void ctx.close()
    },
  }
}
