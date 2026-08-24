import { useEffect, useState } from 'react'
import { initStore, store } from './store'
import Sidebar, { type FilterKey } from './components/Sidebar'
import DownloadList from './components/DownloadList'
import AddModal from './components/AddModal'
import SettingsModal from './components/SettingsModal'

export default function App() {
  const connected = store.connected()
  const [showAdd, setShowAdd] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [filter, setFilter] = useState<FilterKey>('all')

  useEffect(() => {
    initStore().catch((e) => console.error('store init failed', e))
  }, [])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key === 'n') {
        e.preventDefault()
        setShowAdd(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="window-shell">
      <header className="app-titlebar">
        <div className="titlebar-brand">
          <span className="titlebar-logo" aria-hidden="true" />
          <span>ByteRush</span>
        </div>
      </header>
      <div className="app">
        <Sidebar
          filter={filter}
          setFilter={setFilter}
          onAdd={() => setShowAdd(true)}
          onSettings={() => setShowSettings(true)}
        />
        <main className="main">
          {connected ? (
            <DownloadList filter={filter} onAdd={() => setShowAdd(true)} />
          ) : (
            <div className="connecting">
              <div className="spinner" />
              <p>Connecting to download engine...</p>
            </div>
          )}
        </main>
        {showAdd && <AddModal onClose={() => setShowAdd(false)} />}
        {showSettings && <SettingsModal onClose={() => setShowSettings(false)} />}
      </div>
    </div>
  )
}
