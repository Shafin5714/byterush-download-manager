import { store } from '../store'

export type FilterKey = 'all' | 'active' | 'paused' | 'completed' | 'error'

const FILTERS: { key: FilterKey; label: string; icon: string }[] = [
  { key: 'all', label: 'All Downloads', icon: '▤' },
  { key: 'active', label: 'Active', icon: '▶' },
  { key: 'paused', label: 'Paused', icon: '⏸' },
  { key: 'completed', label: 'Completed', icon: '✓' },
  { key: 'error', label: 'Errors', icon: '✕' },
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
        <div className="brand-logo">▼</div>
        <div>
          <h1>ByteRush</h1>
          <p>Download Manager</p>
        </div>
      </div>
      <button className="btn btn-primary btn-block" onClick={onAdd}>
        + New Download
      </button>
      <nav className="nav">
        {FILTERS.map((f) => (
          <button
            key={f.key}
            className={`nav-item ${filter === f.key ? 'active' : ''}`}
            onClick={() => setFilter(f.key)}
          >
            <span className="nav-icon">{f.icon}</span>
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
        <button className="nav-item" onClick={onSettings}>
          <span className="nav-icon">⚙</span>
          <span className="nav-label">Settings</span>
        </button>
      </div>
    </aside>
  )
}
