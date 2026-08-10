import { useCallback, useEffect, useMemo, useState } from 'react'
import type { YoutubeInfo } from '../../shared/types'
import { api } from '../api'
import { store } from '../store'
import { formatBytes, formatDuration, isYouTubeUrl } from '../utils'

interface Props {
  onClose: () => void
}

type Mode = 'direct' | 'youtube' | 'loading' | 'error'

export default function AddModal({ onClose }: Props) {
  const settings = store.settings()
  const [url, setUrl] = useState('')
  const [mode, setMode] = useState<Mode>('direct')
  const [ytInfo, setYtInfo] = useState<YoutubeInfo | null>(null)
  const [ytError, setYtError] = useState('')
  const [filename, setFilename] = useState('')
  const [folder, setFolder] = useState(settings?.downloadDir ?? '')
  const [ytFormat, setYtFormat] = useState('bestvideo*+bestaudio/best')
  const [container, setContainer] = useState('auto')
  const [selectedEntries, setSelectedEntries] = useState<Set<number>>(new Set())
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const isYt = isYouTubeUrl(url)

  const pasteFromClipboard = useCallback(async () => {
    try {
      const text = await navigator.clipboard.readText()
      if (text) setUrl(text.trim())
    } catch {
      /* clipboard unavailable */
    }
  }, [])

  useEffect(() => {
    const t = setTimeout(() => pasteFromClipboard(), 120)
    return () => clearTimeout(t)
  }, [pasteFromClipboard])

  const analyze = async () => {
    if (!url) return
    setMode('loading')
    setYtError('')
    try {
      const info = await api.youtubeInfo(url)
      setYtInfo(info)
      setMode('youtube')
      if (info.isPlaylist && info.entries) {
        setSelectedEntries(new Set(info.entries.map((_, i) => i)))
      }
    } catch (e) {
      setYtError(String(e instanceof Error ? e.message : e))
      setMode('error')
    }
  }

  useEffect(() => {
    if (isYt && mode === 'direct') analyze()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isYt])

  const submit = async () => {
    if (!url || submitting) return
    setSubmitting(true)
    setError('')
    try {
      if (mode === 'youtube' && ytInfo) {
        const items =
          ytInfo.isPlaylist && selectedEntries.size > 0
            ? Array.from(selectedEntries)
                .sort((a, b) => a - b)
                .map((i) => i + 1)
                .join(',')
            : undefined
        const selectedFmt = ytInfo.formats?.find((f) => f.id === ytFormat)
        const effectiveContainer = container !== 'auto' ? container : (selectedFmt?.ext === 'mp4' ? 'mp4' : 'auto')
        await api.youtubeDownload(url, ytFormat, items, folder || undefined, effectiveContainer)
      } else {
        await api.addDownload(url, filename || undefined, folder || undefined)
      }
      onClose()
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e))
    } finally {
      setSubmitting(false)
    }
  }

  const browseFolder = async () => {
    const dir = await window.byterush.chooseDirectory()
    if (dir) setFolder(dir)
  }

  const toggleEntry = (i: number) => {
    setSelectedEntries((prev) => {
      const next = new Set(prev)
      if (next.has(i)) next.delete(i)
      else next.add(i)
      return next
    })
  }

  const presetOptions = useMemo(
    () => [
      { id: 'bestvideo*+bestaudio/best', label: 'Best quality (auto)' },
      { id: 'best', label: 'Best combined (no merge)' },
      { id: 'best[ext=mp4]/best', label: 'MP4 (compatible)' },
      { id: 'bestaudio[ext=m4a]/bestaudio/best', label: 'Audio only (M4A)' },
    ],
    [],
  )

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>New Download</h3>
          <button className="btn btn-icon" onClick={onClose}>
            ✕
          </button>
        </div>
        <div className="modal-body">
          <label className="field-label">URL</label>
          <div className="url-row">
            <input
              className="input"
              placeholder="https://example.com/file.zip or a YouTube link"
              value={url}
              onChange={(e) => {
                setUrl(e.target.value)
                setMode('direct')
                setYtInfo(null)
              }}
              autoFocus
            />
            <button className="btn" onClick={pasteFromClipboard} title="Paste from clipboard">
              📋
            </button>
            {isYt && <button className="btn" onClick={analyze}>Analyze</button>}
          </div>
          {isYt && !ytInfo && mode !== 'loading' && mode !== 'error' && (
            <p className="hint">YouTube link detected — analyzing formats…</p>
          )}
          {mode === 'loading' && <p className="hint">Fetching video info (yt-dlp)…</p>}
          {mode === 'error' && (
            <div className="yt-error">
              <p className="error-text">{ytError}</p>
              <button className="btn" onClick={analyze}>
                Retry analysis
              </button>
            </div>
          )}

          {mode === 'youtube' && ytInfo && (
            <div className="yt-panel">
              <div className="yt-title">
                <strong>{ytInfo.title}</strong>
                {ytInfo.duration ? <span className="yt-duration">{formatDuration(ytInfo.duration)}</span> : null}
                {ytInfo.isPlaylist && (
                  <span className="badge">{ytInfo.entries?.length ?? 0} videos</span>
                )}
              </div>

              {ytInfo.isPlaylist ? (
                <>
                  <label className="field-label">Select videos</label>
                  <div className="yt-entries">
                    {ytInfo.entries?.map((e, i) => (
                      <label key={e.id} className={`yt-entry ${selectedEntries.has(i) ? 'selected' : ''}`}>
                        <input
                          type="checkbox"
                          checked={selectedEntries.has(i)}
                          onChange={() => toggleEntry(i)}
                        />
                        <span className="yt-entry-title">{e.title}</span>
                        {e.duration ? <span className="yt-duration">{formatDuration(e.duration)}</span> : null}
                      </label>
                    ))}
                  </div>
                </>
              ) : (
                <>
                  <label className="field-label">Quality</label>
                  <div className="yt-formats">
                    <label className={`yt-format ${ytFormat === 'bestvideo*+bestaudio/best' ? 'selected' : ''}`}>
                      <input
                        type="radio"
                        name="fmt"
                        checked={ytFormat === 'bestvideo*+bestaudio/best'}
                        onChange={() => setYtFormat('bestvideo*+bestaudio/best')}
                      />
                      <span>
                        <strong>Best quality (auto)</strong>
                        <small>Highest available quality, merged if needed</small>
                      </span>
                    </label>
                    {ytInfo.formats?.map((f) => (
                      <label key={f.id} className={`yt-format ${ytFormat === f.id ? 'selected' : ''}`}>
                        <input type="radio" name="fmt" checked={ytFormat === f.id} onChange={() => setYtFormat(f.id)} />
                        <span>
                          <strong>{f.label}</strong>
                          {f.filesize ? <small>{formatBytes(f.filesize)}</small> : null}
                        </span>
                      </label>
                    ))}
                  </div>
                </>
              )}
            </div>
          )}

          {mode === 'youtube' && ytInfo && (
            <>
              <label className="field-label">File format</label>
              <div className="yt-formats">
                <label className={`yt-format ${container === 'auto' ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="container"
                    checked={container === 'auto'}
                    onChange={() => setContainer('auto')}
                  />
                  <span>
                    <strong>Auto</strong>
                    <small>Keep original container (mp4 / webm / mkv)</small>
                  </span>
                </label>
                <label className={`yt-format ${container === 'mp4' ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="container"
                    checked={container === 'mp4'}
                    onChange={() => setContainer('mp4')}
                  />
                  <span>
                    <strong>MP4</strong>
                    <small>Merge output into .mp4 container</small>
                  </span>
                </label>
              </div>
            </>
          )}

          {mode !== 'youtube' && (
            <>
              <label className="field-label">File name (optional)</label>
              <input
                className="input"
                placeholder="Leave empty to detect automatically"
                value={filename}
                onChange={(e) => setFilename(e.target.value)}
              />
            </>
          )}

          {(mode === 'youtube' || true) && (
            <>
              <label className="field-label">Save to</label>
              <div className="url-row">
                <input className="input" value={folder} onChange={(e) => setFolder(e.target.value)} />
                <button className="btn" onClick={browseFolder}>
                  Browse…
                </button>
              </div>
            </>
          )}

          {mode === 'youtube' && ytInfo?.isPlaylist && (
            <>
              <label className="field-label">Quality for all selected</label>
              <select className="input" value={ytFormat} onChange={(e) => setYtFormat(e.target.value)}>
                {presetOptions.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.label}
                  </option>
                ))}
              </select>
            </>
          )}

          {error && <p className="error-text">{error}</p>}
        </div>
        <div className="modal-footer">
          <button className="btn" onClick={onClose}>
            Cancel
          </button>
          <button
            className="btn btn-primary"
            onClick={submit}
            disabled={
              submitting ||
              !url ||
              (isYt && mode !== 'youtube') ||
              (mode === 'youtube' && ytInfo?.isPlaylist && selectedEntries.size === 0)
            }
          >
            {submitting
              ? 'Adding…'
              : isYt && mode === 'loading'
                ? 'Analyzing…'
                : mode === 'youtube'
                  ? 'Download'
                  : 'Start Download'}
          </button>
        </div>
      </div>
    </div>
  )
}
