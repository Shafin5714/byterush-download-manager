export type DownloadStatus = 'queued' | 'active' | 'paused' | 'completed' | 'error' | 'cancelled'

export interface SegmentState {
  index: number
  start: number
  end: number
  current: number
}

export interface Download {
  id: string
  url: string
  filename: string
  tempFile: string
  finalFile: string
  folder: string
  kind: 'http' | 'youtube'
  totalSize: number
  downloaded: number
  speed: number
  eta: number
  status: DownloadStatus
  segments: SegmentState[]
  error?: string
  createdAt: string
  updatedAt: string
}

export interface Settings {
  downloadDir: string
  maxActive: number
  connections: number
  speedLimitKBs: number
}

export interface YoutubeFormat {
  id: string
  ext: string
  label: string
  height?: number
  fps?: number
  vcodec?: string
  acodec?: string
  filesize?: number
  audio: boolean
}

export interface YoutubeEntry {
  id: string
  title: string
  duration?: number
}

export interface YoutubeInfo {
  title: string
  id: string
  duration?: number
  thumbnail?: string
  isPlaylist: boolean
  entries?: YoutubeEntry[]
  formats?: YoutubeFormat[]
}

export interface EngineEvent {
  type: string
  data: unknown
}

declare global {
  interface Window {
    byterush: {
      getConfig(): Promise<{ port: number; version: string }>
      showWindow(): Promise<void>
      chooseDirectory(): Promise<string | null>
      revealFile(p: string): Promise<void>
      notify(title: string, body: string): Promise<void>
    }
  }
}
