import { spawn } from 'child_process'
import * as fs from 'fs'
import * as path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = path.join(__dirname, '..')
const engineDir = path.join(root, '..', 'engine')
const engineExe = path.join(root, 'resources', 'engine.exe')
const viteUrl = process.env.BYTERUSH_DEV_URL || 'http://localhost:5173'

const children = new Set()

function cleanup() {
  for (const c of children) {
    try {
      if (process.platform === 'win32' && c.pid) {
        spawn('taskkill', ['/pid', String(c.pid), '/T', '/F'], { windowsHide: true, stdio: 'ignore' })
      } else {
        c.kill()
      }
    } catch {
      /* already gone */
    }
  }
}

process.on('exit', cleanup)
process.on('SIGINT', () => {
  cleanup()
  process.exit(0)
})
process.on('SIGTERM', () => {
  cleanup()
  process.exit(0)
})

function engineNeedsBuild() {
  if (!fs.existsSync(engineExe)) return true
  const exeTime = fs.statSync(engineExe).mtimeMs
  const files = []
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, entry.name)
      if (entry.isDirectory()) walk(p)
      else if (entry.name.endsWith('.go')) files.push(p)
    }
  }
  walk(engineDir)
  return files.some((f) => fs.statSync(f).mtimeMs > exeTime)
}

function resolveGo() {
  if (process.env.GO_BIN && fs.existsSync(process.env.GO_BIN)) return process.env.GO_BIN
  const candidates = [
    path.join(process.env.ProgramFiles || '', 'Go', 'bin', 'go.exe'),
    path.join(process.env.LOCALAPPDATA || '', 'Programs', 'Go', 'bin', 'go.exe'),
  ]
  for (const c of candidates) {
    if (fs.existsSync(c)) return c
  }
  return process.env.GO_BIN || 'go'
}

async function buildEngine() {
  if (!engineNeedsBuild()) {
    console.log('[dev] engine is up to date')
    return
  }
  console.log('[dev] building engine...')
  await new Promise((resolve, reject) => {
    fs.mkdirSync(path.dirname(engineExe), { recursive: true })
    const go = resolveGo()
    const proc = spawn(go, ['build', '-o', engineExe, '.'], {
      cwd: engineDir,
      stdio: 'inherit',
      windowsHide: true,
    })
    proc.on('error', reject)
    proc.on('exit', (code) => (code === 0 ? resolve() : reject(new Error('engine build failed'))))
  })
  console.log('[dev] engine built')
}

async function waitFor(url, timeoutMs = 60000) {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(url)
      if (res.ok) return
    } catch {
      /* not ready */
    }
    await new Promise((r) => setTimeout(r, 300))
  }
  throw new Error(`timed out waiting for ${url}`)
}

async function main() {
  await buildEngine()

  // invoke the node CLIs directly — no shell, no .cmd resolution issues
  const nodeBin = process.execPath
  const viteCli = path.join(root, 'node_modules', 'vite', 'bin', 'vite.js')
  const electronCli = path.join(root, 'node_modules', 'electron', 'cli.js')

  console.log('[dev] starting vite...')
  const vite = spawn(nodeBin, [viteCli], { cwd: root, stdio: 'inherit', windowsHide: true })
  children.add(vite)
  await waitFor(viteUrl)

  console.log('[dev] starting electron...')
  const electron = spawn(nodeBin, [electronCli, '.'], {
    cwd: root,
    stdio: 'inherit',
    windowsHide: true,
    env: { ...process.env, BYTERUSH_DEV_URL: viteUrl },
  })
  children.add(electron)

  vite.on('exit', () => {
    cleanup()
    process.exit(0)
  })
}

main().catch((e) => {
  console.error(e)
  cleanup()
  process.exit(1)
})
