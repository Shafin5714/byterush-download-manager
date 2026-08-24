import { app, BrowserWindow, dialog, ipcMain, Menu, nativeImage, Notification, shell, Tray } from 'electron'
import { spawn, ChildProcess } from 'child_process'
import * as fs from 'fs'
import * as http from 'http'
import * as path from 'path'

const DEV_URL = process.env.BYTERUSH_DEV_URL || ''
const SMOKE = process.env.BYTERUSH_SMOKE === '1'
const isDev = DEV_URL !== ''

let engine: ChildProcess | null = null
let enginePort = 0
let tray: Tray | null = null
let win: BrowserWindow | null = null

const gotLock = app.requestSingleInstanceLock()
if (!gotLock) {
  app.quit()
} else {
  app.on('second-instance', () => {
    if (win) {
      if (win.isMinimized()) win.restore()
      win.focus()
    }
  })
}

function enginePath(): string {
  if (app.isPackaged) return path.join(process.resourcesPath, 'engine.exe')
  return path.join(app.getAppPath(), 'resources', 'engine.exe')
}

function startEngine(): Promise<number> {
  return new Promise((resolve, reject) => {
    const exe = enginePath()
    if (!fs.existsSync(exe)) {
      reject(new Error(`engine binary not found at ${exe} — run "npm run build:engine"`))
      return
    }
    const dataDir = path.join(app.getPath('userData'), 'engine')
    const child = spawn(exe, ['--port', '29641', '--dir', dataDir], {
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe'],
    })
    engine = child
    let settled = false
    let out = ''
    child.stdout.on('data', (d: Buffer) => {
      out += d.toString()
      const m = out.match(/LISTENING (\d+)/)
      if (m && !settled) {
        settled = true
        enginePort = parseInt(m[1], 10)
        resolve(enginePort)
      }
    })
    child.stderr.on('data', (d: Buffer) => console.error('[engine]', d.toString().trim()))
    child.on('exit', (code) => {
      console.error('[engine] exited with code', code)
      if (!settled) {
        settled = true
        reject(new Error(`engine exited with code ${code}`))
      }
    })
    setTimeout(() => {
      if (!settled) {
        settled = true
        reject(new Error('engine start timeout'))
      }
    }, 15000)
  })
}

function enginePost(p: string, body?: unknown): Promise<void> {
  return new Promise((resolve) => {
    if (!enginePort) {
      resolve()
      return
    }
    const req = http.request(
      { host: '127.0.0.1', port: enginePort, path: p, method: 'POST', headers: { 'Content-Type': 'application/json' } },
      (res) => {
        res.resume()
        res.on('end', () => resolve())
      },
    )
    req.on('error', () => resolve())
    req.setTimeout(1500, () => {
      req.destroy()
      resolve()
    })
    req.end(body ? JSON.stringify(body) : undefined)
  })
}

function createWindow() {
  win = new BrowserWindow({
    width: 1120,
    height: 740,
    minWidth: 820,
    minHeight: 560,
    backgroundColor: '#0f1115',
    title: 'ByteRush',
    icon: path.join(app.getAppPath(), 'resources', 'icon-256.png'),
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, '../preload/preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  })
  if (isDev) {
    win.loadURL(DEV_URL)
  } else {
    win.loadFile(path.join(app.getAppPath(), 'dist', 'index.html'))
  }
  win.on('closed', () => {
    win = null
  })
  if (SMOKE) {
    win.webContents.on('did-finish-load', async () => {
      const html = await win!.webContents.executeJavaScript(
        'document.getElementById("root").innerHTML.length + "|" + document.title + "|" + document.querySelectorAll("div").length',
      )
      const screenshotPath = process.env.BYTERUSH_SMOKE_SCREENSHOT
      if (screenshotPath) {
        await new Promise((resolve) => setTimeout(resolve, 500))
        const action = process.env.BYTERUSH_SMOKE_ACTION
        if (action === 'add') {
          await win!.webContents.executeJavaScript('document.querySelector(".add-button")?.click()')
        } else if (action === 'settings') {
          await win!.webContents.executeJavaScript('document.querySelector(".sidebar-footer .nav-item")?.click()')
        }
        await new Promise((resolve) => setTimeout(resolve, 300))
        const image = await win!.webContents.capturePage()
        fs.writeFileSync(screenshotPath, image.toPNG())
        console.log('SMOKE_SCREENSHOT', screenshotPath)
      }
      console.log('SMOKE_OK', html)
      app.quit()
      setTimeout(() => process.exit(0), 800)
    })
    win.webContents.on('did-fail-load', (_e, code, desc) => {
      console.error('SMOKE_FAIL', code, desc)
      app.exit(1)
    })
  }
}

function createTray() {
  const iconPath = path.join(app.getAppPath(), 'resources', 'icon-16.png')
  let icon = nativeImage.createFromPath(iconPath)
  if (icon.isEmpty()) icon = nativeImage.createEmpty()
  tray = new Tray(icon)
  tray.setToolTip('ByteRush')
  tray.setContextMenu(
    Menu.buildFromTemplate([
      { label: 'Open ByteRush', click: () => {
        if (win) {
          win.show()
          win.focus()
        }
      } },
      { type: 'separator' },
      { label: 'Pause All', click: () => enginePost('/api/downloads/pause-all') },
      { label: 'Resume All', click: () => enginePost('/api/downloads/resume-all') },
      { type: 'separator' },
      { label: 'Quit', click: () => app.quit() },
    ]),
  )
}

function registerIpc() {
  ipcMain.handle('byterush:config', () => ({ port: enginePort, version: app.getVersion() }))
  ipcMain.handle('byterush:choose-directory', async () => {
    const res = await dialog.showOpenDialog(win!, { properties: ['openDirectory', 'createDirectory'] })
    return res.canceled || res.filePaths.length === 0 ? null : res.filePaths[0]
  })
  ipcMain.handle('byterush:reveal-file', (_e, p: string) => {
    shell.showItemInFolder(p)
  })
  ipcMain.handle('byterush:notify', (_e, title: string, body: string) => {
    if (Notification.isSupported()) new Notification({ title, body }).show()
  })
}

app.whenReady().then(async () => {
  app.setAppUserModelId('com.byterush.app')
  registerIpc()
  try {
    enginePort = await startEngine()
  } catch (err) {
    dialog.showErrorBox('ByteRush engine failed', String(err))
    app.quit()
    return
  }
  createWindow()
  if (!SMOKE && process.platform === 'win32') createTray()
})

app.on('before-quit', () => {
  enginePost('/api/shutdown')
  const child = engine
  engine = null
  setTimeout(() => {
    if (child) child.kill()
  }, 800)
})

app.on('window-all-closed', () => {
  app.quit()
})
