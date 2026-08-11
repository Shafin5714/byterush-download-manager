const ENGINE = 'http://127.0.0.1:29641'

async function ping() {
  try {
    const res = await fetch(ENGINE + '/api/ping')
    return res.ok
  } catch {
    return false
  }
}

async function sendToEngine(url, filename) {
  const res = await fetch(ENGINE + '/api/downloads', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url, filename: filename || undefined }),
  })
  if (!res.ok) throw new Error('engine error ' + res.status)
  return res.json()
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
  return false
})

// Intercept browser downloads and hand them to ByteRush when enabled.
chrome.downloads.onCreated.addListener(async (item) => {
  try {
    const { autoCapture } = await chrome.storage.local.get('autoCapture')
    if (!autoCapture) return
    if (!item.url || (!item.url.startsWith('http://') && !item.url.startsWith('https://'))) return
    await sendToEngine(item.url, item.filename)
    try {
      await chrome.downloads.cancel(item.id)
      await chrome.downloads.erase({ id: item.id })
    } catch {
      /* download already finished or erased */
    }
  } catch {
    // engine unreachable or server rejected — let the browser download proceed
  }
})

// Make sure the service worker stays alive while the popup talks to it.
chrome.runtime.onSuspend?.addListener(() => {})
