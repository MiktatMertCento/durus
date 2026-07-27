export type Status = 'idle' | 'running' | 'paused'
export type Phase = 'sitting' | 'standing' | 'rest'

export type SessionState = {
  id?: string
  status: Status
  phase?: Phase
  nextPhase?: Phase
  phaseDurationSec: number
  remainingSec: number
  elapsedSec: number
  overtimeSec: number
  awaitingAdvance: boolean
  transitionCount: number
  startedAt?: string
  updatedAt: string
}

export type SessionListItem = {
  id: string
  startedAt: string
  endedAt: string
  totalHuman: string
  workHuman: string
  lateHuman: string
  workedHours: number
  transitions: number
  lateMin: number
}

export type PhaseRecord = {
  phase: Phase
  startedAt: string
  endedAt: string
  plannedSec: number
  actualSec: number
  lateSec: number
  lateMin: number
  confirmed: boolean
}

export type SessionArchive = {
  id: string
  startedAt: string
  endedAt: string
  phases: PhaseRecord[]
  totals: {
    durationSec: number
    durationMin: number
    workSec: number
    restSec: number
    sittingSec: number
    standingSec: number
    lateSec: number
    lateMin: number
    pausedSec: number
    pauseCount: number
    transitionCount: number
    phaseCounts: Record<string, number>
    avgLateSec: number
  }
  summary: {
    startedAt: string
    endedAt: string
    totalHuman: string
    workHuman: string
    restHuman: string
    lateHuman: string
    workedHours: number
    transitions: number
    onTimeAdvances: number
    lateAdvances: number
  }
}

export type ServerMessage =
  | { type: 'state'; state: SessionState }
  | { type: 'error'; message: string }

export const idleState: SessionState = {
  status: 'idle',
  phaseDurationSec: 0,
  remainingSec: 0,
  elapsedSec: 0,
  overtimeSec: 0,
  awaitingAdvance: false,
  transitionCount: 0,
  updatedAt: '',
}

export const phaseLabel: Record<Phase, string> = {
  sitting: 'Oturarak çalış',
  standing: 'Ayakta çalış',
  rest: 'Dinlen',
}

export const phaseHint: Record<Phase, string> = {
  sitting: '40 dakika',
  standing: '15 dakika',
  rest: '5 dakika',
}

export const advanceLabel: Record<Phase, string> = {
  sitting: 'Ayağa kalktım',
  standing: 'Dinlenmeye geçtim',
  rest: 'Oturmaya geçtim',
}

export const phaseShort: Record<Phase, string> = {
  sitting: 'otur',
  standing: 'ayak',
  rest: 'dinlen',
}

export function documentTitleFor(state: SessionState): string {
  if (state.status === 'idle' || !state.phase) {
    return 'duruş'
  }
  const clock = formatClock(state.remainingSec, state.overtimeSec)
  const phase = phaseShort[state.phase]
  const pause = state.status === 'paused' ? ' · durak' : ''
  const wait = state.awaitingAdvance ? ' · geç' : ''
  return `${clock} · ${phase}${pause}${wait} · duruş`
}

export function formatClock(remainingSec: number, overtimeSec: number): string {
  if (overtimeSec > 0 || remainingSec < 0) {
    return `+${formatTime(Math.abs(overtimeSec || -remainingSec))}`
  }
  return formatTime(remainingSec)
}

export function formatTime(totalSec: number): string {
  const sec = Math.max(0, Math.ceil(totalSec))
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) {
    return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  }
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

export function wsUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/ws`
}
