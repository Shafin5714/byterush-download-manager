import { useState } from 'react'
import type { Download } from '../../shared/types'
import { api } from '../api'
import { store } from '../store'
import { formatBytes, formatEta, formatSpeed, percent } from '../utils'
import { FilterKey } from './Sidebar'

const STATUS_META: Record<string, { label: string; cls: string; icon: string }> = {
  queued: { label: 'Queued', cls: 'status-queued', icon: '⏳' },
  active: { label: 'Active', cls: 'status-active', icon: '▶' },
  paused: { label: 'Paused', cls: 'status-paused', icon: '⏸' },
  completed: { label: 'Completed', cls: 'status-completed', icon: '✓' },
  error: { label: 'Error', cls: 'status-error', icon: '✕' },
}

function DownloadRow({ d }: { d: Download }) {
  const pct = percent(d)
  const meta = STATUS_META[d.status] ?? STATUS_META.queued
  const [busy, setBusy] = useState(false)

  const act = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    try {
      await fn()
    } catch (e) {
      console.error(e)
    } finally {
      setBusy(false)
    }
  }

  const isRunning = d.status === 'active' || d.status === 'queued'

  return (
    <div className={`row ${d.status}`}>
      <div className="row-icon" title={meta.label}>
        {meta.icon}
      </div>
      <div className="row-main">
        <div className="row-top">
          <span className="row-name" title={d.filename}>
            {d.kind === 'youtube' && <span className="badge">YT</span>}
            {d.filename || '(unknown)'}
          </span>
          <span className="row-percent">
            {d.totalSize > 0 && d.status !== 'completed' && d.status !== 'error'
              ? `${pct.toFixed(1)}%`
              : ''}
          </span>
        </div>
        <div className="row-url" title={d.url}>
          {d.url}
        </div>
        <div className="progress">
          <div
            className={`progress-fill ${d.status}`}
            style={{ width: `${d.status === 'active' && d.totalSize === 0 ? '100' : pct}%` }}
          >
            {d.status === 'active' && d.totalSize === 0 && <div className="progress-indeterminate" />}
          </div>
        </div>
        <div className="row-meta">
          {isRunning && (
            <>
              <span className="meta-speed">{formatSpeed(d.speed)}</span>
              <span className="meta-size">
                {formatBytes(d.downloaded)}
                {d.totalSize > 0 ? ` / ${formatBytes(d.totalSize)}` : ''}
              </span>
              {d.eta > 0 && <span className="meta-eta">ETA {formatEta(d.eta)}</span>}
              <span className="meta-segs">{d.segments.length} connections</span>
            </>
          )}
          {d.status === 'paused' && (
            <span className="meta-size">
              {formatBytes(d.downloaded)} / {formatBytes(d.totalSize)}
            </span>
          )}
          {d.status === 'completed' && (
            <span className="meta-size">Saved to {d.folder}</span>
          )}
          {d.status === 'error' && <span className="meta-error">{d.error || 'Download failed'}</span>}
        </div>
      </div>
      <div className="row-actions">
        {isRunning && (
          <button className="btn btn-icon" title="Pause" onClick={() => act(() => api.pause(d.id))} disabled={busy}>
            ⏸
          </button>
        )}
        {(d.status === 'paused' || d.status === 'error') && (
          <button className="btn btn-icon" title="Resume" onClick={() => act(() => api.resume(d.id))} disabled={busy}>
            ▶
          </button>
        )}
        {d.status === 'completed' && (
          <button
            className="btn btn-icon"
            title="Show in folder"
            onClick={() => window.byterush.revealFile(d.finalFile || d.tempFile)}
          >
            📂
          </button>
        )}
        <button className="btn btn-icon btn-danger" title="Remove" onClick={() => act(() => api.cancel(d.id))} disabled={busy}>
          ✕
        </button>
      </div>
    </div>
  )
}

interface Props {
  filter: FilterKey
  onAdd: () => void
}

export default function DownloadList({ filter, onAdd }: Props) {
  const list = store.list()
  const [query, setQuery] = useState('')

  const filtered = list.filter((d) => {
    if (filter === 'active' && !(d.status === 'active' || d.status === 'queued')) return false
    if (filter === 'paused' && d.status !== 'paused') return false
    if (filter === 'completed' && d.status !== 'completed') return false
    if (filter === 'error' && d.status !== 'error') return false
    if (query && !d.filename.toLowerCase().includes(query.toLowerCase()) && !d.url.toLowerCase().includes(query.toLowerCase()))
      return false
    return true
  })

  return (
    <div className="list-view">
      <header className="list-header">
        <div>
          <h2>{filter === 'all' ? 'Downloads' : filter.charAt(0).toUpperCase() + filter.slice(1)}</h2>
          <p className="list-subtitle">
            {list.filter((d) => d.status === 'active' || d.status === 'queued').length} active ·{' '}
            {list.filter((d) => d.status === 'completed').length} completed
          </p>
        </div>
        <div className="list-actions">
          <input
            className="search"
            placeholder="Search downloads…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <button
            className="btn"
            onClick={() => {
              const active = list.filter((d) => d.status === 'active' || d.status === 'queued')
              active.forEach((d) => api.pause(d.id))
            }}
          >
            ⏸ Pause all
          </button>
          <button
            className="btn"
            onClick={() => {
              list.filter((d) => d.status === 'paused').forEach((d) => api.resume(d.id))
            }}
          >
            ▶ Resume all
          </button>
          <button className="btn btn-primary" onClick={onAdd}>
            + Add
          </button>
        </div>
      </header>
      {filtered.length === 0 ? (
        <div className="empty">
          <div className="empty-icon">▼</div>
          <h3>No downloads here yet</h3>
          <p>Paste a link or click “New Download” to start.</p>
        </div>
      ) : (
        <div className="rows">
          {filtered.map((d) => (
            <DownloadRow key={d.id} d={d} />
          ))}
        </div>
      )}
    </div>
  )
}
