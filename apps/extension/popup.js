const statusEl = document.getElementById('status')
const urlEl = document.getElementById('url')
const sendBtn = document.getElementById('send')
const autoEl = document.getElementById('auto')

async function refreshStatus() {
  try {
    const res = await chrome.runtime.sendMessage({ type: 'ping' })
    statusEl.textContent = res && res.ok ? 'Engine online' : 'Engine offline'
    statusEl.className = 'status ' + (res && res.ok ? 'on' : 'off')
  } catch {
    statusEl.textContent = 'Engine offline'
    statusEl.className = 'status off'
  }
}

chrome.storage.local.get('autoCapture', (data) => {
  autoEl.checked = !!data.autoCapture
})

autoEl.addEventListener('change', () => {
  chrome.storage.local.set({ autoCapture: autoEl.checked })
})

urlEl.addEventListener('focus', async () => {
  try {
    const text = await navigator.clipboard.readText()
    if (text && /^https?:\/\//i.test(text.trim())) urlEl.value = text.trim()
  } catch {
    /* clipboard unavailable */
  }
})

urlEl.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') sendUrl()
})

sendBtn.addEventListener('click', sendUrl)

async function sendUrl() {
  const url = urlEl.value.trim()
  if (!url) return
  sendBtn.disabled = true
  sendBtn.textContent = 'Sending…'
  try {
    const res = await chrome.runtime.sendMessage({ type: 'send-url', url })
    if (res && res.ok) {
      urlEl.value = ''
      sendBtn.textContent = 'Sent to ByteRush ✓'
    } else {
      sendBtn.textContent = 'Failed — is ByteRush running?'
    }
  } catch {
    sendBtn.textContent = 'Failed — is ByteRush running?'
  }
  setTimeout(() => {
    sendBtn.disabled = false
    sendBtn.textContent = 'Send to ByteRush'
  }, 1500)
}

refreshStatus()
setInterval(refreshStatus, 3000)
