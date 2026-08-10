import { useEffect, useState } from 'react'
import type { Settings } from '../../shared/types'
import { api } from '../api'
import { store } from '../store'

interface Props {
  onClose: () => void
}

export default function SettingsModal({ onClose }: Props) {
  const current = store.settings()
  const [form, setForm] = useState<Settings | null>(current)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (!form && current) setForm(current)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current])

  if (!form) return null

  const set = (patch: Partial<Settings>) => setForm((f) => (f ? { ...f, ...patch } : f))

  const save = async () => {
    setSaving(true)
    setError('')
    try {
      await api.saveSettings(form)
      setSaved(true)
      setTimeout(onClose, 400)
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e))
    } finally {
      setSaving(false)
    }
  }

  const browse = async () => {
    const dir = await window.byterush.chooseDirectory()
    if (dir) set({ downloadDir: dir })
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal modal-sm" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Settings</h3>
          <button className="btn btn-icon" onClick={onClose}>
            ✕
          </button>
        </div>
        <div className="modal-body">
          <label className="field-label">Default download folder</label>
          <div className="url-row">
            <input className="input" value={form.downloadDir} onChange={(e) => set({ downloadDir: e.target.value })} />
            <button className="btn" onClick={browse}>
              Browse…
            </button>
          </div>

          <label className="field-label">
            Concurrent downloads <span className="hint-inline">({form.maxActive})</span>
          </label>
          <input
            type="range"
            min={1}
            max={10}
            value={form.maxActive}
            onChange={(e) => set({ maxActive: Number(e.target.value) })}
          />

          <label className="field-label">
            Connections per file <span className="hint-inline">({form.connections})</span>
          </label>
          <input
            type="range"
            min={1}
            max={32}
            value={form.connections}
            onChange={(e) => set({ connections: Number(e.target.value) })}
          />

          <label className="field-label">
            Global speed limit <span className="hint-inline">{form.speedLimitKBs > 0 ? `${form.speedLimitKBs} KB/s` : 'Unlimited'}</span>
          </label>
          <input
            type="range"
            min={0}
            max={10000}
            step={100}
            value={form.speedLimitKBs}
            onChange={(e) => set({ speedLimitKBs: Number(e.target.value) })}
          />
          <p className="hint">0 = unlimited.</p>

          {error && <p className="error-text">{error}</p>}
          {saved && <p className="ok-text">Settings saved.</p>}
        </div>
        <div className="modal-footer">
          <button className="btn" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-primary" onClick={save} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  )
}
