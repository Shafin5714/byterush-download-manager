import { store } from '../store'
import Icon, { type IconName } from './Icon'

export type FilterKey = 'all' | 'active' | 'paused' | 'completed' | 'error'

const FILTERS: { key: FilterKey; label: string; icon: IconName }[] = [
  { key: 'all', label: 'All downloads', icon: 'grid' },
  { key: 'active', label: 'Active', icon: 'activity' },
  { key: 'paused', label: 'Paused', icon: 'pause' },
  { key: 'completed', label: 'Completed', icon: 'check' },
  { key: 'error', label: 'Needs attention', icon: 'alert' },
]

interface Props {
  filter: FilterKey
  setFilter: (f: FilterKey) => void
  onAdd: () => void
  onSettings: () => void
}

export default function Sidebar({ filter, setFilter, onAdd, onSettings }: Props) {
  const list = store.list()
  const logs = store.logs()
  const counts: Record<FilterKey, number> = {
    all: list.length,
    active: list.filter((d) => d.status === 'active' || d.status === 'queued').length,
    paused: list.filter((d) => d.status === 'paused').length,
    completed: list.filter((d) => d.status === 'completed').length,
    error: list.filter((d) => d.status === 'error').length,
  }
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-logo" aria-hidden="true" />
        <div className="brand-copy">
          <h1>ByteRush</h1>
          <p>Download manager</p>
        </div>
        <span className="brand-version">v0.1</span>
      </div>
      <button className="btn btn-primary btn-block" onClick={onAdd}>
        <Icon name="plus" size={17} />
        <span>New download</span>
        <kbd>Ctrl N</kbd>
      </button>
      <p className="nav-section-label">Library</p>
      <nav className="nav">
        {FILTERS.map((f) => (
          <button
            key={f.key}
            className={`nav-item ${filter === f.key ? 'active' : ''}`}
            onClick={() => setFilter(f.key)}
          >
            <span className="nav-icon"><Icon name={f.icon} size={17} /></span>
            <span className="nav-label">{f.label}</span>
            <span className="nav-count">{counts[f.key]}</span>
          </button>
        ))}
      </nav>
      <div className="sidebar-footer">
        {logs.length > 0 && (
          <div className="sidebar-logs">
            {logs.slice(-4).map((l, i) => (
              <div key={i} className="log-line" title={l}>
                {l}
              </div>
            ))}
          </div>
        )}
        <div className="engine-status">
          <span className="status-dot" />
          <span><strong>Engine online</strong><small>Ready for downloads</small></span>
        </div>
        <button className="nav-item" onClick={onSettings}>
          <span className="nav-icon"><Icon name="settings" size={17} /></span>
          <span className="nav-label">Settings</span>
        </button>
      </div>
    </aside>
  )
}
