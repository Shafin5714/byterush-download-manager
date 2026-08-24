const ENGINE = 'http://127.0.0.1:29641'

async function ping() {
  try {
    const res = await fetch(ENGINE + '/api/ping')
    return res.ok
  } catch {
    return false
  }
}

function filenameOnly(filename) {
  if (!filename) return undefined
  return filename.split(/[\\/]/).pop() || undefined
}

async function browserRequestHeaders(url, referrer) {
  const headers = {
    Accept: '*/*',
    'User-Agent': navigator.userAgent,
  }
  if (referrer) headers.Referer = referrer
  try {
    const cookies = await chrome.cookies.getAll({ url })
    if (cookies.length) headers.Cookie = cookies.map((cookie) => `${cookie.name}=${cookie.value}`).join('; ')
  } catch {
    // Cookie access can be unavailable for a URL; the engine may still be able to fetch it.
  }
  return headers
}

async function sendToEngine(url, filename, referrer) {
  const requestHeaders = await browserRequestHeaders(url, referrer)
  return engineRequest('/api/downloads', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url, filename: filenameOnly(filename), requestHeaders }),
  })
}

async function engineRequest(path, options) {
  const res = await fetch(ENGINE + path, options)
  let data
  try {
    data = await res.json()
  } catch {
    data = null
  }
  if (!res.ok) throw new Error(data?.error || `ByteRush returned HTTP ${res.status}`)
  return data
}

function youtubeWatchUrl(value) {
  try {
    const url = new URL(value)
    const host = url.hostname.toLowerCase()
    if (url.protocol !== 'https:' || (host !== 'youtube.com' && host !== 'www.youtube.com')) return null
    const videoId = url.pathname === '/watch' ? url.searchParams.get('v') : ''
    if (!videoId || !/^[\w-]{6,20}$/.test(videoId)) return null
    return `https://www.youtube.com/watch?v=${videoId}`
  } catch {
    return null
  }
}

async function youtubeInfo(url) {
  return engineRequest('/api/youtube/info', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url }),
  })
}

async function youtubeDownload(url, format, container) {
  return engineRequest('/api/youtube/download', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      url,
      format: typeof format === 'string' && format.length <= 200 ? format : 'bestvideo*+bestaudio/best',
      container: container === 'mp4' ? 'mp4' : 'auto',
    }),
  })
}

async function removeEngineDownload(id) {
  try {
    await fetch(`${ENGINE}/api/downloads/${id}/cancel`, { method: 'POST' })
  } catch {
    // The engine may have exited; there is nothing else to clean up.
  }
}

async function waitForEngineStart(id, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const res = await fetch(ENGINE + '/api/downloads')
    if (!res.ok) throw new Error('engine status error ' + res.status)
    const downloads = await res.json()
    const download = downloads.find((entry) => entry.id === id)
    if (!download) return 'failed'
    if (download.status === 'completed' || download.downloaded > 0) return 'started'
    if (download.status === 'error' || download.status === 'cancelled') return 'failed'
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  return 'timeout'
}

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg && msg.type === 'ping') {
    ping().then((ok) => sendResponse({ ok }))
    return true
  }
  if (msg && msg.type === 'send-url') {
    sendToEngine(msg.url, msg.filename)
      .then(() => sendResponse({ ok: true }))
      .catch((e) => sendResponse({ ok: false, error: String(e) }))
    return true
  }
  if (msg && msg.type === 'youtube-info') {
    const url = youtubeWatchUrl(msg.url)
    if (!url) {
      sendResponse({ ok: false, error: 'Open a YouTube video before downloading.' })
      return false
    }
    youtubeInfo(url)
      .then((info) => sendResponse({ ok: true, info }))
      .catch((e) => sendResponse({ ok: false, error: String(e instanceof Error ? e.message : e) }))
    return true
  }
  if (msg && msg.type === 'youtube-download') {
    const url = youtubeWatchUrl(msg.url)
    if (!url) {
      sendResponse({ ok: false, error: 'Open a YouTube video before downloading.' })
      return false
    }
    youtubeDownload(url, msg.format, msg.container)
      .then((download) => sendResponse({ ok: true, download }))
      .catch((e) => sendResponse({ ok: false, error: String(e instanceof Error ? e.message : e) }))
    return true
  }
  return false
})

// Intercept browser downloads and hand them to ByteRush when enabled.
chrome.downloads.onCreated.addListener(async (item) => {
  let paused = false
  let engineDownload = null
  try {
    const { autoCapture } = await chrome.storage.local.get('autoCapture')
    if (!autoCapture) return
    if (!item.url || (!item.url.startsWith('http://') && !item.url.startsWith('https://'))) return

    // Keep Chrome's authenticated request available until ByteRush proves that
    // it can fetch the file. This matters for Cloudflare-protected hosts.
    await chrome.downloads.pause(item.id)
    paused = true
    const [freshItem] = await chrome.downloads.search({ id: item.id })
    const downloadUrl = freshItem?.finalUrl || item.finalUrl || freshItem?.url || item.url
    const referrer = freshItem?.referrer || item.referrer
    engineDownload = await sendToEngine(downloadUrl, freshItem?.filename || item.filename, referrer)
    const handoff = await waitForEngineStart(engineDownload.id)
    if (handoff !== 'started') {
      if (handoff === 'timeout') await removeEngineDownload(engineDownload.id)
      await chrome.downloads.resume(item.id)
      return
    }

    try {
      await chrome.downloads.cancel(item.id)
      await chrome.downloads.erase({ id: item.id })
    } catch {
      // The browser download may already have completed or been erased.
    }
  } catch {
    if (engineDownload?.id) await removeEngineDownload(engineDownload.id)
    if (paused) {
      try {
        await chrome.downloads.resume(item.id)
      } catch {
        // The browser download may already have completed or been removed.
      }
    }
  }
})

// Make sure the service worker stays alive while the popup talks to it.
chrome.runtime.onSuspend?.addListener(() => {})
