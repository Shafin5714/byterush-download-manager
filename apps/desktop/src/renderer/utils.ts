export function formatBytes(n: number): string {
  if (!n || n < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${i === 0 || v >= 100 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

export function formatSpeed(s: number): string {
  return s > 0 ? `${formatBytes(s)}/s` : ''
}

export function formatEta(sec: number): string {
  if (sec <= 0) return ''
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

export function isYouTubeUrl(url: string): boolean {
  return /(youtube\.com|youtu\.be)/i.test(url)
}

export function percent(d: { downloaded: number; totalSize: number }): number {
  if (d.totalSize > 0) return Math.min(100, (d.downloaded / d.totalSize) * 100)
  return d.downloaded > 0 ? 100 : 0
}

export function formatDuration(sec?: number): string {
  if (!sec || sec <= 0) return ''
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}
