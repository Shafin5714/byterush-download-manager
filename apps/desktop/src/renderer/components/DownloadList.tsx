import { useState } from 'react'
import type { Download } from '../../shared/types'
import { api } from '../api'
import { store } from '../store'
import { formatBytes, formatEta, formatSpeed, percent } from '../utils'
import type { FilterKey } from './Sidebar'
import Icon, { type IconName } from './Icon'

const STATUS_META: Record<string, { label: string; icon: IconName }> = {
  queued: { label: 'Queued', icon: 'download' },
  active: { label: 'Downloading', icon: 'activity' },
  paused: { label: 'Paused', icon: 'pause' },
  completed: { label: 'Completed', icon: 'check' },
  error: { label: 'Needs attention', icon: 'alert' },
}

const FILTER_TITLES: Record<FilterKey, string> = {
  all: 'Downloads',
  active: 'Active downloads',
  paused: 'Paused downloads',
  completed: 'Completed downloads',
  error: 'Needs attention',
}

function fileType(d: Download): string {
  if (d.kind === 'youtube') return 'YT'
  const name = d.filename || ''
  const dot = name.lastIndexOf('.')
  if (dot < 0 || dot === name.length - 1) return 'FILE'
  return name.slice(dot + 1).toUpperCase().slice(0, 4)
}

function DownloadRow({ d }: { d: Download }) {
  const pct = percent(d)
  const meta = STATUS_META[d.status] ?? STATUS_META.queued
  const [busy, setBusy] = useState(false)
  const isRunning = d.status === 'active' || d.status === 'queued'
  const isIndeterminate = d.status === 'active' && d.totalSize <= 0

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

  return (
    <article className={`download-card ${d.status}`}>
      <div className="file-tile" aria-hidden="true">
        <Icon name={meta.icon} size={20} />
        <span>{fileType(d)}</span>
      </div>

      <div className="download-content">
        <div className="download-title-row">
          <div className="download-name-wrap">
            <h3 className="download-name" title={d.filename}>{d.filename || 'Preparing download...'}</h3>
            <span className={`status-pill ${d.status}`}><span />{meta.label}</span>
          </div>
          {d.totalSize > 0 && d.status !== 'error' && (
            <strong className="download-percent">{d.status === 'completed' ? '100%' : `${pct.toFixed(1)}%`}</strong>
          )}
        </div>

        <div className="download-url" title={d.url}><Icon name="link" size={13} />{d.url}</div>

        <div className="progress-track" aria-label={`${pct.toFixed(1)}% downloaded`}>
          <div
            className={`progress-value ${d.status} ${isIndeterminate ? 'indeterminate' : ''}`}
            style={{ width: `${isIndeterminate ? 100 : d.status === 'completed' ? 100 : pct}%` }}
          />
        </div>

        <div className="download-meta">
          {d.status === 'queued' && <span>Waiting for an available slot</span>}
          {d.status === 'active' && (
            <>
              <strong className="speed-value">{formatSpeed(d.speed)}</strong>
              <span>{formatBytes(d.downloaded)}{d.totalSize > 0 ? ` of ${formatBytes(d.totalSize)}` : ''}</span>
              {d.eta > 0 && <span>{formatEta(d.eta)} remaining</span>}
              <span>{Math.max(d.segments.length, 1)} connection{d.segments.length === 1 ? '' : 's'}</span>
            </>
          )}
          {d.status === 'paused' && <span>{formatBytes(d.downloaded)} of {formatBytes(d.totalSize)} downloaded</span>}
          {d.status === 'completed' && <span className="saved-path"><Icon name="folder" size={13} />Saved to {d.folder}</span>}
          {d.status === 'error' && <span className="meta-error">{d.error || 'Download failed'}</span>}
        </div>
      </div>

      <div className="download-actions">
        {isRunning && (
          <button className="icon-button" title="Pause download" aria-label="Pause download" onClick={() => act(() => api.pause(d.id))} disabled={busy}>
            <Icon name="pause" size={17} />
          </button>
        )}
        {(d.status === 'paused' || d.status === 'error') && (
          <button className="icon-button primary-action" title="Resume download" aria-label="Resume download" onClick={() => act(() => api.resume(d.id))} disabled={busy}>
            <Icon name="play" size={17} />
          </button>
        )}
        {d.status === 'completed' && (
          <button className="icon-button" title="Show in folder" aria-label="Show in folder" onClick={() => window.byterush.revealFile(d.finalFile || d.tempFile)}>
            <Icon name="folder" size={17} />
          </button>
        )}
        <button className="icon-button danger-action" title="Remove from list" aria-label="Remove from list" onClick={() => act(() => api.cancel(d.id))} disabled={busy}>
          <Icon name="trash" size={17} />
        </button>
      </div>
    </article>
  )
}

interface Props {
  filter: FilterKey
  onAdd: () => void
}

export default function DownloadList({ filter, onAdd }: Props) {
  const list = store.list()
  const [query, setQuery] = useState('')
  const activeCount = list.filter((d) => d.status === 'active' || d.status === 'queued').length
  const pausedCount = list.filter((d) => d.status === 'paused').length
  const completedCount = list.filter((d) => d.status === 'completed').length

  const normalizedQuery = query.trim().toLowerCase()
  const filtered = list.filter((d) => {
    if (filter === 'active' && !(d.status === 'active' || d.status === 'queued')) return false
    if (filter === 'paused' && d.status !== 'paused') return false
    if (filter === 'completed' && d.status !== 'completed') return false
    if (filter === 'error' && d.status !== 'error') return false
    if (normalizedQuery && !d.filename.toLowerCase().includes(normalizedQuery) && !d.url.toLowerCase().includes(normalizedQuery)) return false
    return true
  })

  const emptyTitle = normalizedQuery
    ? 'No matching downloads'
    : filter === 'all'
      ? 'Ready for your first download'
      : `No ${filter === 'error' ? 'downloads need attention' : `${filter} downloads`}`
  const emptyDescription = normalizedQuery
    ? 'Try a different filename or URL.'
    : filter === 'all'
      ? 'Add a direct link or send a download from your browser to get started.'
      : 'Items will appear here when their status changes.'

  return (
    <section className="list-view">
      <header className="page-header">
        <div className="page-heading">
          <p className="eyebrow">Download library</p>
          <h2>{FILTER_TITLES[filter]}</h2>
          <div className="summary-line">
            <span><i className="summary-dot active" />{activeCount} active</span>
            <span><i className="summary-dot completed" />{completedCount} completed</span>
          </div>
        </div>
        <button className="btn btn-primary add-button" onClick={onAdd}>
          <Icon name="plus" size={17} />Add download
        </button>
      </header>

      <div className="toolbar">
        <label className="search-field">
          <Icon name="search" size={17} />
          <input placeholder="Search by filename or URL" value={query} onChange={(e) => setQuery(e.target.value)} />
          {query && <button onClick={() => setQuery('')} title="Clear search" aria-label="Clear search"><Icon name="x" size={14} /></button>}
        </label>
        <div className="toolbar-actions">
          <button className="btn btn-secondary" onClick={() => api.pauseAll()} disabled={activeCount === 0}>
            <Icon name="pause" size={16} />Pause all
          </button>
          <button className="btn btn-secondary" onClick={() => api.resumeAll()} disabled={pausedCount === 0}>
            <Icon name="play" size={16} />Resume all
          </button>
        </div>
      </div>

      {filtered.length === 0 ? (
        <div className="empty-state">
          <div className="empty-glow" />
          <div className="empty-visual"><Icon name="download" size={34} /></div>
          <h3>{emptyTitle}</h3>
          <p>{emptyDescription}</p>
          {filter === 'all' && !normalizedQuery && (
            <button className="btn btn-primary empty-cta" onClick={onAdd}><Icon name="plus" size={17} />New download</button>
          )}
          {normalizedQuery && <button className="btn btn-secondary" onClick={() => setQuery('')}>Clear search</button>}
          <span className="keyboard-hint">Tip: press <kbd>Ctrl</kbd> <kbd>N</kbd> anywhere</span>
        </div>
      ) : (
        <div className="download-list">
          {filtered.map((d) => <DownloadRow key={d.id} d={d} />)}
        </div>
      )}
    </section>
  )
}
