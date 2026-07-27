import { useEffect, useId, useRef } from 'react'

type Props = {
  open: boolean
  title: string
  body: string
  confirmLabel?: string
  cancelLabel?: string
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({
  open,
  title,
  body,
  confirmLabel = 'Kapat',
  cancelLabel = 'Vazgeç',
  onConfirm,
  onCancel,
}: Props) {
  const titleId = useId()
  const bodyId = useId()
  const confirmRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    confirmRef.current?.focus()

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onCancel])

  if (!open) return null

  return (
    <div className="dialog-root" role="presentation">
      <button type="button" className="dialog-backdrop" aria-label="Kapat" onClick={onCancel} />
      <div
        className="dialog"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={bodyId}
      >
        <p id={titleId} className="dialog-title">
          {title}
        </p>
        <p id={bodyId} className="dialog-body">
          {body}
        </p>
        <div className="dialog-actions">
          <button type="button" className="btn ghost" onClick={onCancel}>
            {cancelLabel}
          </button>
          <button ref={confirmRef} type="button" className="btn primary" onClick={onConfirm}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
