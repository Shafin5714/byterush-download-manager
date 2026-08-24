const BYTERUSH_BUTTON_ID = 'byterush-youtube-action'
const BYTERUSH_PANEL_ID = 'byterush-youtube-panel'
const DEFAULT_FORMAT = 'bestvideo*+bestaudio/best'

let analyzedUrl = ''
let currentInfo = null
let requestSequence = 0

function currentVideoUrl() {
  if (location.pathname !== '/watch') return ''
  const videoId = new URL(location.href).searchParams.get('v')
  return videoId ? `https://www.youtube.com/watch?v=${videoId}` : ''
}

function actionContainer() {
  const selectors = [
    'ytd-watch-metadata #top-level-buttons-computed',
    'ytd-watch-metadata #actions-inner #top-level-buttons-computed',
    'ytd-watch-flexy #menu #top-level-buttons-computed',
  ]
  for (const selector of selectors) {
    const element = document.querySelector(selector)
    if (element) return element
  }
  return null
}

function shareButtonHost(container) {
  const candidates = container.querySelectorAll('button, [role="button"]')
  for (const candidate of candidates) {
    const label = `${candidate.getAttribute('aria-label') || ''} ${candidate.getAttribute('title') || ''}`
    if (!/^\s*share\b/i.test(label)) continue
    let host = candidate
    while (host.parentElement && host.parentElement !== container) host = host.parentElement
    return host.parentElement === container ? host : null
  }
  return null
}

function createDownloadIcon() {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
  svg.setAttribute('viewBox', '0 0 24 24')
  svg.setAttribute('aria-hidden', 'true')
  const path = document.createElementNS('http://www.w3.org/2000/svg', 'path')
  path.setAttribute('d', 'M11 4h2v9l3.5-3.5 1.4 1.4-5.9 5.9-5.9-5.9 1.4-1.4L11 13V4Zm-5 15h12v2H6v-2Z')
  svg.appendChild(path)
  return svg
}

function createActionButton() {
  const slot = document.createElement('span')
  slot.id = BYTERUSH_BUTTON_ID
  slot.className = 'byterush-youtube-action'

  const button = document.createElement('button')
  button.type = 'button'
  button.className = 'byterush-youtube-button'
  button.title = 'Download this video with ByteRush'
  button.setAttribute('aria-label', 'Download this video with ByteRush')
  button.append(createDownloadIcon(), document.createTextNode('Download'))
  button.addEventListener('click', (event) => {
    event.preventDefault()
    event.stopPropagation()
    togglePanel(button)
  })

  slot.appendChild(button)
  return slot
}

function syncButton() {
  const videoUrl = currentVideoUrl()
  const existing = document.getElementById(BYTERUSH_BUTTON_ID)
  if (!videoUrl) {
    existing?.remove()
    closePanel()
    return
  }
  if (existing?.isConnected) return

  const container = actionContainer()
  if (!container) return
  const button = createActionButton()
  const shareHost = shareButtonHost(container)
  if (shareHost?.parentElement === container) shareHost.insertAdjacentElement('afterend', button)
  else container.appendChild(button)
}

function makeElement(tag, className, text) {
  const element = document.createElement(tag)
  if (className) element.className = className
  if (text !== undefined) element.textContent = text
  return element
}

function createPanel(anchor) {
  const panel = makeElement('section', 'byterush-youtube-panel')
  if (youtubeUsesDarkTheme()) panel.classList.add('is-dark')
  panel.id = BYTERUSH_PANEL_ID
  panel.setAttribute('role', 'dialog')
  panel.setAttribute('aria-label', 'Download with ByteRush')

  const header = makeElement('div', 'byterush-youtube-panel-header')
  const brand = makeElement('div', 'byterush-youtube-brand')
  const logo = makeElement('span', 'byterush-youtube-logo', 'B')
  const title = makeElement('div', '', 'Download with ByteRush')
  const close = makeElement('button', 'byterush-youtube-close', '×')
  close.type = 'button'
  close.setAttribute('aria-label', 'Close')
  close.addEventListener('click', closePanel)
  brand.append(logo, title)
  header.append(brand, close)

  const body = makeElement('div', 'byterush-youtube-panel-body')
  panel.append(header, body)
  document.body.appendChild(panel)
  positionPanel(anchor, panel)
  return panel
}

function youtubeUsesDarkTheme() {
  if (document.documentElement.hasAttribute('dark') || document.querySelector('ytd-app[dark]')) return true
  const color = getComputedStyle(document.body).backgroundColor.match(/[\d.]+/g)?.slice(0, 3).map(Number)
  return !!color && color.length === 3 && color[0] * 0.299 + color[1] * 0.587 + color[2] * 0.114 < 128
}

function positionPanel(anchor, panel = document.getElementById(BYTERUSH_PANEL_ID)) {
  if (!anchor?.isConnected || !panel) return
  const rect = anchor.getBoundingClientRect()
  const width = Math.min(360, window.innerWidth - 24)
  panel.style.width = `${width}px`
  const left = Math.max(12, Math.min(rect.right - width, window.innerWidth - width - 12))
  const measuredHeight = panel.offsetHeight || 260
  const below = rect.bottom + 10
  const top = below + measuredHeight <= window.innerHeight - 12 ? below : Math.max(12, rect.top - measuredHeight - 10)
  panel.style.left = `${Math.round(left)}px`
  panel.style.top = `${Math.round(top)}px`
}

function setButtonBusy(busy) {
  const button = document.querySelector(`#${BYTERUSH_BUTTON_ID} button`)
  if (!button) return
  button.disabled = busy
  button.classList.toggle('is-busy', busy)
}

function showLoading(panel) {
  const body = panel.querySelector('.byterush-youtube-panel-body')
  body.replaceChildren()
  const loading = makeElement('div', 'byterush-youtube-loading')
  loading.append(makeElement('span', 'byterush-youtube-spinner'), makeElement('span', '', 'Checking available formats…'))
  body.appendChild(loading)
  positionPanel(document.querySelector(`#${BYTERUSH_BUTTON_ID} button`), panel)
}

function showError(panel, message, anchor) {
  const body = panel.querySelector('.byterush-youtube-panel-body')
  body.replaceChildren()
  body.append(
    makeElement('p', 'byterush-youtube-error', message || 'ByteRush could not analyze this video.'),
    makeElement('p', 'byterush-youtube-hint', 'Make sure the ByteRush desktop app is running, then try again.'),
  )
  const retry = makeElement('button', 'byterush-youtube-primary', 'Try again')
  retry.type = 'button'
  retry.addEventListener('click', () => analyzeVideo(panel, anchor, true))
  body.appendChild(retry)
  setButtonBusy(false)
  requestAnimationFrame(() => positionPanel(anchor, panel))
}

function selectField(labelText, className) {
  const field = makeElement('label', 'byterush-youtube-field')
  field.appendChild(makeElement('span', 'byterush-youtube-field-label', labelText))
  const select = makeElement('select', className)
  field.appendChild(select)
  return { field, select }
}

function addOption(select, value, label, audio = false, ext = '') {
  const option = makeElement('option', '', label)
  option.value = value
  option.dataset.audio = audio ? 'true' : 'false'
  option.dataset.ext = ext
  select.appendChild(option)
}

function showFormats(panel, info, videoUrl, anchor) {
  const body = panel.querySelector('.byterush-youtube-panel-body')
  body.replaceChildren()

  const videoTitle = makeElement('p', 'byterush-youtube-video-title', info.title || 'Current YouTube video')
  videoTitle.title = info.title || ''
  body.appendChild(videoTitle)

  const qualityField = selectField('Quality', 'byterush-youtube-select byterush-youtube-quality')
  addOption(qualityField.select, DEFAULT_FORMAT, 'Best quality (auto)')
  addOption(qualityField.select, 'best[ext=mp4]/best', 'MP4 (compatible)', false, 'mp4')
  addOption(qualityField.select, 'bestaudio[ext=m4a]/bestaudio/best', 'Audio only (M4A)', true, 'm4a')

  const seen = new Set(Array.from(qualityField.select.options, (option) => option.value))
  for (const format of info.formats || []) {
    if (!format?.id || seen.has(format.id)) continue
    seen.add(format.id)
    addOption(
      qualityField.select,
      format.id,
      format.label || `${format.height || 'Video'} · ${format.ext || 'auto'}`,
      !!format.audio,
      format.ext || '',
    )
  }

  const containerField = selectField('File format', 'byterush-youtube-select byterush-youtube-container')
  addOption(containerField.select, 'auto', 'Auto (recommended)')
  addOption(containerField.select, 'mp4', 'MP4')

  const mp4Option = containerField.select.querySelector('option[value="mp4"]')
  qualityField.select.addEventListener('change', () => {
    const selected = qualityField.select.selectedOptions[0]
    const audioOnly = selected?.dataset.audio === 'true'
    mp4Option.disabled = audioOnly
    if (audioOnly) containerField.select.value = 'auto'
  })

  const download = makeElement('button', 'byterush-youtube-primary', 'Download')
  download.type = 'button'
  download.addEventListener('click', async () => {
    download.disabled = true
    download.textContent = 'Adding to ByteRush…'
    try {
      const selectedQuality = qualityField.select.selectedOptions[0]
      const effectiveContainer =
        containerField.select.value === 'auto' && selectedQuality?.dataset.ext === 'mp4' && selectedQuality.dataset.audio !== 'true'
          ? 'mp4'
          : containerField.select.value
      const response = await chrome.runtime.sendMessage({
        type: 'youtube-download',
        url: videoUrl,
        format: qualityField.select.value,
        container: effectiveContainer,
      })
      if (!response?.ok) throw new Error(response?.error || 'ByteRush rejected the download.')
      download.classList.add('is-success')
      download.textContent = 'Added to ByteRush ✓'
      setTimeout(closePanel, 1400)
    } catch (error) {
      download.disabled = false
      download.textContent = 'Download'
      showInlineError(body, error instanceof Error ? error.message : String(error))
    }
  })

  body.append(qualityField.field, containerField.field, download)
  setButtonBusy(false)
  requestAnimationFrame(() => positionPanel(anchor, panel))
}

function showInlineError(body, message) {
  body.querySelector('.byterush-youtube-inline-error')?.remove()
  const error = makeElement('p', 'byterush-youtube-error byterush-youtube-inline-error', message)
  body.insertBefore(error, body.lastElementChild)
}

async function analyzeVideo(panel, anchor, force = false) {
  const videoUrl = currentVideoUrl()
  if (!videoUrl) {
    closePanel()
    return
  }
  if (!force && analyzedUrl === videoUrl && currentInfo) {
    showFormats(panel, currentInfo, videoUrl, anchor)
    return
  }

  const sequence = ++requestSequence
  showLoading(panel)
  setButtonBusy(true)
  try {
    const response = await chrome.runtime.sendMessage({ type: 'youtube-info', url: videoUrl })
    if (sequence !== requestSequence || !panel.isConnected) return
    if (!response?.ok) throw new Error(response?.error || 'ByteRush could not analyze this video.')
    analyzedUrl = videoUrl
    currentInfo = response.info
    showFormats(panel, response.info, videoUrl, anchor)
  } catch (error) {
    if (sequence !== requestSequence || !panel.isConnected) return
    showError(panel, error instanceof Error ? error.message : String(error), anchor)
  }
}

function togglePanel(anchor) {
  const existing = document.getElementById(BYTERUSH_PANEL_ID)
  if (existing) {
    closePanel()
    return
  }
  const panel = createPanel(anchor)
  analyzeVideo(panel, anchor)
}

function closePanel() {
  requestSequence++
  document.getElementById(BYTERUSH_PANEL_ID)?.remove()
  setButtonBusy(false)
}

function handleOutsidePointer(event) {
  const panel = document.getElementById(BYTERUSH_PANEL_ID)
  const action = document.getElementById(BYTERUSH_BUTTON_ID)
  if (panel && !panel.contains(event.target) && !action?.contains(event.target)) closePanel()
}

function handleKeydown(event) {
  if (event.key === 'Escape') closePanel()
}

function handleViewportChange() {
  const anchor = document.querySelector(`#${BYTERUSH_BUTTON_ID} button`)
  positionPanel(anchor)
}

document.addEventListener('yt-navigate-finish', () => {
  analyzedUrl = ''
  currentInfo = null
  closePanel()
  setTimeout(syncButton, 100)
})
document.addEventListener('pointerdown', handleOutsidePointer, true)
document.addEventListener('keydown', handleKeydown, true)
window.addEventListener('resize', handleViewportChange)
window.addEventListener('scroll', handleViewportChange, true)

syncButton()
setInterval(syncButton, 1500)
