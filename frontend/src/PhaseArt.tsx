import { useEffect, useState } from 'react'
import type { Phase } from './types.ts'
import sittingUrl from './assets/phase/sitting.svg'
import standingUrl from './assets/phase/standing.svg'
import restUrl from './assets/phase/rest.svg'
import idleUrl from './assets/phase/idle.svg'

const marks: Record<Phase | 'idle', string> = {
  sitting: sittingUrl,
  standing: standingUrl,
  rest: restUrl,
  idle: idleUrl,
}

type Props = {
  phase: Phase | 'idle'
  active: boolean
}

export function PhaseArt({ phase, active }: Props) {
  const kind = active ? phase : 'idle'
  const [shown, setShown] = useState(kind)
  const [visible, setVisible] = useState(true)

  useEffect(() => {
    if (kind === shown) return
    setVisible(false)
    const t = window.setTimeout(() => {
      setShown(kind)
      setVisible(true)
    }, 240)
    return () => window.clearTimeout(t)
  }, [kind, shown])

  return (
    <div className={`phase-art kind-${shown}`} aria-hidden="true">
      <img
        className={`mark ${visible ? 'is-in' : 'is-out'}`}
        src={marks[shown]}
        alt=""
        draggable={false}
      />
    </div>
  )
}
