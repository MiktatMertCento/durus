import { useEffect, useState } from 'react'
import type { SessionArchive, SessionListItem } from './types.ts'
import { phaseLabel, type Phase } from './types.ts'

function formatClockTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString('tr-TR', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function HistoryPage() {
  const [items, setItems] = useState<SessionListItem[]>([])
  const [selected, setSelected] = useState<SessionArchive | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    document.title = 'Geçmiş · duruş'
    return () => {
      document.title = 'duruş'
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const res = await fetch('/api/sessions')
        if (!res.ok) throw new Error('Liste alınamadı')
        const data = (await res.json()) as SessionListItem[]
        if (!cancelled) setItems(data)
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Hata')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const openDetail = async (id: string) => {
    setError('')
    try {
      const res = await fetch(`/api/sessions/${id}`)
      if (!res.ok) throw new Error('Session bulunamadı')
      setSelected((await res.json()) as SessionArchive)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Hata')
    }
  }

  return (
    <div className="history">
      <header className="history-head">
        <div>
          <p className="brand-mark">duruş</p>
          <h1>Geçmiş session’lar</h1>
        </div>
        <a className="btn ghost" href="#/">
          Timer’a dön
        </a>
      </header>

      {loading ? <p className="sub">Yükleniyor…</p> : null}
      {error ? <p className="history-error">{error}</p> : null}

      {!loading && items.length === 0 ? (
        <p className="sub">Henüz kapatılmış session yok. Timer’ı kapatınca burada listelenir.</p>
      ) : null}

      <div className="history-layout">
        <ul className="history-list">
          {items.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                className={`history-row ${selected?.id === item.id ? 'on' : ''}`}
                onClick={() => openDetail(item.id)}
              >
                <span className="history-when">
                  {item.startedAt}
                  <span>→ {item.endedAt}</span>
                </span>
                <span className="history-meta">
                  <strong>{item.workHuman}</strong> çalışma
                  <em>{item.lateHuman} geç</em>
                </span>
              </button>
            </li>
          ))}
        </ul>

        {selected ? (
          <article className="history-detail">
            <h2>
              {selected.summary.startedAt}
              <span> → {selected.summary.endedAt}</span>
            </h2>
            <dl className="stat-grid">
              <div>
                <dt>Toplam süre</dt>
                <dd>{selected.summary.totalHuman}</dd>
              </div>
              <div>
                <dt>Çalışma</dt>
                <dd>{selected.summary.workHuman}</dd>
              </div>
              <div>
                <dt>Dinlenme</dt>
                <dd>{selected.summary.restHuman}</dd>
              </div>
              <div>
                <dt>Geç kalınan</dt>
                <dd>{selected.summary.lateHuman}</dd>
              </div>
              <div>
                <dt>Saat cinsinden</dt>
                <dd>{selected.summary.workedHours} sa</dd>
              </div>
              <div>
                <dt>Faz geçişi</dt>
                <dd>
                  {selected.summary.transitions} ({selected.summary.onTimeAdvances} zamanında /{' '}
                  {selected.summary.lateAdvances} geç)
                </dd>
              </div>
            </dl>

            <h3>Fazlar</h3>
            <ul className="phase-list">
              {selected.phases.map((p, i) => (
                <li key={`${p.phase}-${i}`}>
                  <strong>{phaseLabel[p.phase as Phase] ?? p.phase}</strong>
                  <span>
                    {formatClockTime(p.startedAt)} → {formatClockTime(p.endedAt)}
                  </span>
                  <span>
                    {Math.round(p.actualSec / 60)} dk
                    {p.lateMin > 0 ? ` · +${p.lateMin} dk geç` : ' · zamanında'}
                    {!p.confirmed ? ' · session kapandı' : ''}
                  </span>
                </li>
              ))}
            </ul>
          </article>
        ) : null}
      </div>
    </div>
  )
}
