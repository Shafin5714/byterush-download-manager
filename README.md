# ByteRush — Download Manager

A fast, IDM-style download manager for Windows, built with **Electron + React + TypeScript** on the frontend and a **Go download engine** on the backend, with **yt-dlp** powering YouTube downloads.

> ⚠️ **Legal note:** YouTube's Terms of Service prohibit downloading videos without authorization. Use the YouTube features only for content you own or have explicit permission to download.

## Features

- **Segmented downloads** — files are split into chunks (default 8 connections) and fetched in parallel for maximum speed, like IDM.
- **Resume / pause** — progress is tracked per segment and survives app restarts.
- **Queue manager** — concurrent-download limits, priority ordering, retry with exponential backoff.
- **Speed limiter** — global bandwidth cap (token bucket).
- **YouTube** — paste a video or playlist URL, pick quality/format, download single videos or selected playlist items. yt-dlp is auto-downloaded on first use.
- **Browser bridge** — optional Chrome/Edge extension that sends downloads to ByteRush (auto-capture or manual).
- **System tray** — pause all / resume all / quick open.
- **Persistent history** — downloads are stored in JSON under `%APPDATA%/byterush/engine`.

## Architecture

```
┌───────────────────────────────────────┐
│        Electron + React UI            │   queue list, progress, modals
└──────────────────┬────────────────────┘
                   │  HTTP + SSE (127.0.0.1:<port>)
┌──────────────────▼────────────────────┐
│          Go download engine           │
│  segmented downloader · resume ·      │
│  throttle · queue · yt-dlp wrapper    │
└───────────────────────────────────────┘
                   ▲
      Browser extension (MV3) ──┘
```

The engine listens on `127.0.0.1:29641` (falls back to the next free port) and exposes a small JSON API:

| Endpoint | Purpose |
|---|---|
| `POST /api/downloads` | Add a download `{url, filename?, folder?}` |
| `GET /api/downloads` | List downloads |
| `POST /api/downloads/{id}/pause` `/resume` `/cancel` | Control a download |
| `POST /api/downloads/pause-all` `/resume-all` | Bulk control |
| `GET/POST /api/settings` | Get/update settings |
| `POST /api/youtube/info` | Fetch formats/playlist entries via yt-dlp |
| `POST /api/youtube/download` | Start a YouTube download |
| `GET /api/events` | SSE stream of progress/status events |
| `POST /api/shutdown` | Graceful shutdown |

## Repository layout

```
apps/
├── desktop/     Electron + React + TypeScript app (UI, main process, preload)
├── engine/      Go download engine (single binary, stdlib only)
└── extension/   Chrome/Edge MV3 extension (browser capture bridge)
```

## Requirements

- Windows 10/11
- Node.js 20+ and npm
- Go 1.22+ (only to build the engine)

## Development

```powershell
# 1. install desktop dependencies
cd apps/desktop
npm install

# 2. run in dev mode (builds engine, starts Vite + Electron with hot reload)
npm run dev
```

## Building

```powershell
cd apps/desktop
npm run build        # engine + renderer + main process
npm start            # run the built app
npm run dist         # produce a Windows NSIS installer in apps/desktop/release/
```

The built app bundles `resources/engine.exe`; yt-dlp is fetched automatically on first YouTube use.

## Testing the engine alone

```powershell
cd apps/engine
go build -o bin/engine.exe .
powershell -ExecutionPolicy Bypass -File test.ps1   # runs download/pause/resume/throttle/persistence tests
```

## Installing the browser extension

1. Open `edge://extensions` (or `chrome://extensions`).
2. Enable **Developer mode**.
3. Click **Load unpacked** and select `apps/extension/`.
4. Open the ByteRush popup — it shows whether the engine is online and lets you toggle auto-capture.

ByteRush must be running for the extension to hand downloads off. Capture targets the engine on `127.0.0.1:29641`.

## Roadmap

- [ ] Per-download connection count and speed limit
- [ ] Scheduler (time windows, bandwidth schedules)
- [ ] Auto-sort into category folders (Videos / Music / Documents…)
- [ ] Firefox extension port
- [ ] Auto-update for the app itself

## License

MIT — see [LICENSE](LICENSE).
