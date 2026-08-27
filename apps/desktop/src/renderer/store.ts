import { useSyncExternalStore } from 'react'
import type { Download, Settings } from '../shared/types'
import { api, connectEvents, initApi } from './api'

interface State {
  downloads: Record<string, Download>
  list: Download[]
  settings: Settings | null
  connected: boolean
  logs: string[]
}

let state: State = { downloads: {}, list: [], settings: null, connected: false, logs: [] }
const listeners = new Set<() => void>()

function emit() {
  listeners.forEach((l) => l())
}

function mutate(fn: (s: State) => void) {
  const next: State = { ...state, downloads: { ...state.downloads }, logs: [...state.logs] }
  fn(next)
  next.list = Object.values(next.downloads).sort((a, b) => b.createdAt.localeCompare(a.createdAt))
  state = next
  emit()
}

const completedNotified = new Set<string>()

export async function initStore(): Promise<void> {
  await initApi()
  connectEvents((e) => {
    if (e.type === 'added' || e.type === 'update') {
      const d = e.data as Download
      mutate((s) => {
        s.downloads[d.id] = d
      })
      if (e.type === 'added') {
        void window.byterush.showWindow()
      }
      if (e.type === 'update' && d.status === 'completed' && !completedNotified.has(d.id)) {
        completedNotified.add(d.id)
        window.byterush.notify('Download complete', d.filename)
      }
    } else if (e.type === 'removed') {
      const id = e.data as string
      mutate((s) => {
        delete s.downloads[id]
      })
    } else if (e.type === 'settings') {
      mutate((s) => {
        s.settings = e.data as Settings
      })
    } else if (e.type === 'log') {
      mutate((s) => {
        s.logs.push(String(e.data))
        if (s.logs.length > 200) s.logs.shift()
      })
    }
  })
  const [list, settings] = await Promise.all([api.listDownloads(), api.getSettings()])
  mutate((s) => {
    const map: Record<string, Download> = {}
    for (const d of list) map[d.id] = d
    s.downloads = map
    s.settings = settings
    s.connected = true
  })
}

function subscribe(cb: () => void) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

function useStore<T>(selector: (s: State) => T): T {
  return useSyncExternalStore(subscribe, () => selector(state))
}

export const store = {
  list: () => useStore((s) => s.list),
  settings: () => useStore((s) => s.settings),
  connected: () => useStore((s) => s.connected),
  logs: () => useStore((s) => s.logs),
}
