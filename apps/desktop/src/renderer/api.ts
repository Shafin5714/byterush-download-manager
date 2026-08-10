import type { Download, EngineEvent, Settings, YoutubeInfo } from '../shared/types'

let base = ''

export async function initApi(): Promise<void> {
  const cfg = await window.byterush.getConfig()
  base = `http://127.0.0.1:${cfg.port}`
}

async function req<T>(method: string, p: string, body?: unknown): Promise<T> {
  const res = await fetch(base + p, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    let msg = res.statusText
    try {
      const j = await res.json()
      if (j && j.error) msg = j.error
    } catch {
      /* ignore */
    }
    throw new Error(msg)
  }
  return res.json() as Promise<T>
}

export const api = {
  listDownloads: () => req<Download[]>('GET', '/api/downloads'),
  addDownload: (url: string, filename?: string, folder?: string) =>
    req<Download>('POST', '/api/downloads', { url, filename, folder }),
  pause: (id: string) => req<unknown>('POST', `/api/downloads/${id}/pause`),
  resume: (id: string) => req<unknown>('POST', `/api/downloads/${id}/resume`),
  cancel: (id: string) => req<unknown>('POST', `/api/downloads/${id}/cancel`),
  pauseAll: () => req<unknown>('POST', '/api/downloads/pause-all'),
  resumeAll: () => req<unknown>('POST', '/api/downloads/resume-all'),
  getSettings: () => req<Settings>('GET', '/api/settings'),
  saveSettings: (s: Partial<Settings>) => req<Settings>('POST', '/api/settings', s),
  youtubeInfo: (url: string) => req<YoutubeInfo>('POST', '/api/youtube/info', { url }),
  youtubeDownload: (url: string, format: string, playlistItems?: string, folder?: string, container?: string) =>
    req<Download>('POST', '/api/youtube/download', { url, format, playlistItems, folder, container }),
}

export function connectEvents(onEvent: (e: EngineEvent) => void): () => void {
  const es = new EventSource(base + '/api/events')
  es.onmessage = (msg) => {
    try {
      onEvent(JSON.parse(msg.data))
    } catch {
      /* ignore */
    }
  }
  return () => es.close()
}
