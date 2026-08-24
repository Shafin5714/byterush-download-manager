import { useEffect, useState } from 'react'
import type { Settings } from '../../shared/types'
import { api } from '../api'
import { store } from '../store'
import Icon from './Icon'

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
          <div className="modal-title">
            <div className="modal-title-icon"><Icon name="settings" size={19} /></div>
            <div><h3>Settings</h3><p>Control download performance and storage.</p></div>
          </div>
          <button className="icon-button modal-close" onClick={onClose} aria-label="Close settings">
            <Icon name="x" size={18} />
          </button>
        </div>
        <div className="modal-body">
          <div className="settings-section">
            <div className="setting-heading"><Icon name="folder" size={18} /><span><strong>Download location</strong><small>Choose where completed files are saved.</small></span></div>
            <div className="url-row">
              <input className="input" value={form.downloadDir} onChange={(e) => set({ downloadDir: e.target.value })} />
              <button className="btn btn-secondary" onClick={browse}><Icon name="folder" size={16} />Browse</button>
            </div>
          </div>

          <div className="settings-section performance-settings">
            <div className="setting-heading"><Icon name="activity" size={18} /><span><strong>Performance</strong><small>Balance speed with network and disk usage.</small></span></div>
            <label className="range-setting">
              <span><strong>Concurrent downloads</strong><output>{form.maxActive}</output></span>
              <input type="range" min={1} max={10} value={form.maxActive} onChange={(e) => set({ maxActive: Number(e.target.value) })} />
            </label>
            <label className="range-setting">
              <span><strong>Connections per file</strong><output>{form.connections}</output></span>
              <input type="range" min={1} max={32} value={form.connections} onChange={(e) => set({ connections: Number(e.target.value) })} />
            </label>
            <label className="range-setting">
              <span><strong>Global speed limit</strong><output>{form.speedLimitKBs > 0 ? `${form.speedLimitKBs} KB/s` : 'Unlimited'}</output></span>
              <input type="range" min={0} max={10000} step={100} value={form.speedLimitKBs} onChange={(e) => set({ speedLimitKBs: Number(e.target.value) })} />
            </label>
          </div>

          {error && <p className="error-text">{error}</p>}
          {saved && <p className="ok-text">Settings saved.</p>}
        </div>
        <div className="modal-footer">
          <button className="btn" onClick={onClose}>
            Cancel
          </button>
          <button className="btn btn-primary" onClick={save} disabled={saving}>
            {saving ? 'Saving...' : 'Save changes'}
          </button>
        </div>
      </div>
    </div>
  )
}
