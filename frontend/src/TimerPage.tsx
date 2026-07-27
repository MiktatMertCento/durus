import { useEffect, useState } from 'react'
import { useSession } from './useSession.ts'
import {
  advanceLabel,
  documentTitleFor,
  formatClock,
  phaseHint,
  phaseLabel,
  type Phase,
} from './types.ts'
import { PhaseArt } from './PhaseArt.tsx'
import { ConfirmDialog } from './ConfirmDialog.tsx'

export function TimerPage() {
  const { state, connected, error, start, stop, close, advance } = useSession()
  const [closeOpen, setCloseOpen] = useState(false)
  const active = state.status === 'running' || state.status === 'paused'
  const phase: Phase = state.phase ?? 'sitting'
  const nextPhase: Phase = state.nextPhase ?? 'standing'
  const awaiting = state.awaitingAdvance
  const progress =
    state.phaseDurationSec > 0
      ? Math.min(1, state.elapsedSec / state.phaseDurationSec)
      : 0

  useEffect(() => {
    document.title = documentTitleFor(state)
    return () => {
      document.title = 'duruş'
    }
  }, [state])

  useEffect(() => {
    if (!active) setCloseOpen(false)
  }, [active])

  return (
    <div
      className={`app phase-${active ? phase : 'idle'}${awaiting ? ' is-awaiting' : ''}`}
      data-status={state.status}
    >
      <div className="atmosphere" aria-hidden="true" />

      <header className="brand">
        <p className="brand-mark">duruş</p>
        <div className="brand-right">
          <a className="nav-link" href="#/gecmis">
            Geçmiş
          </a>
          <p className={`link ${connected ? 'ok' : 'bad'}`}>
            {connected ? 'bağlı' : 'yeniden bağlanıyor…'}
          </p>
        </div>
      </header>

      <div className="layout">
        <main className="stage">
          {!active ? (
            <>
              <h1 className="headline">Masaya otur, ritmi başlat.</h1>
              <p className="sub">
                40 dk oturarak · 15 dk ayakta · 5 dk dinlenme. Her cihazdan yönet.
              </p>
            </>
          ) : (
            <>
              <p className="phase-tag">{phaseLabel[phase]}</p>
              <p className={`clock${awaiting ? ' overtime' : ''}`} aria-live="polite">
                {formatClock(state.remainingSec, state.overtimeSec)}
              </p>
              <p className="sub">
                {awaiting
                  ? `Süre doldu · +${Math.floor(state.overtimeSec / 60)} dk geç · sıradaki: ${phaseLabel[nextPhase]}`
                  : `${state.status === 'paused' ? 'Duraklatıldı · ' : ''}${phaseHint[phase]}`}
              </p>
              <div
                className="bar"
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={Math.round(progress * 100)}
              >
                <span style={{ width: `${progress * 100}%` }} />
              </div>
            </>
          )}

          {error ? <p className="inline-error">{error}</p> : null}

          <div className="actions">
            {awaiting ? (
              <button
                type="button"
                className="btn primary pulse"
                onClick={advance}
                disabled={!connected}
              >
                {advanceLabel[phase]}
              </button>
            ) : null}

            {!active || state.status === 'paused' ? (
              <button
                type="button"
                className="btn primary"
                onClick={start}
                disabled={!connected}
              >
                {state.status === 'paused' ? 'Devam et' : 'Başlat'}
              </button>
            ) : (
              <button type="button" className="btn" onClick={stop} disabled={!connected}>
                Durdur
              </button>
            )}

            {active ? (
              <button
                type="button"
                className="btn ghost"
                onClick={() => setCloseOpen(true)}
                disabled={!connected}
              >
                Kapat
              </button>
            ) : null}
          </div>
        </main>

        <PhaseArt phase={phase} active={active} />
      </div>

      <footer className="cycle">
        <span className={phase === 'sitting' && active ? 'on' : ''}>otur</span>
        <span className={phase === 'standing' && active ? 'on' : ''}>ayak</span>
        <span className={phase === 'rest' && active ? 'on' : ''}>dinlen</span>
      </footer>

      <ConfirmDialog
        open={closeOpen}
        title="Session kapatılsın mı?"
        body="Bu oturum sonlanır ve geçmişe kaydedilir. Sayaç sıfırlanır."
        confirmLabel="Evet, kapat"
        cancelLabel="Vazgeç"
        onCancel={() => setCloseOpen(false)}
        onConfirm={() => {
          setCloseOpen(false)
          close()
        }}
      />
    </div>
  )
}
