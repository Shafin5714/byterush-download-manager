# ByteRush — Download Manager

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform: Windows](https://img.shields.io/badge/Platform-Windows%2010%20%7C%2011-0078D6?logo=windows)](https://github.com/Shafin5714/byterush-download-manager)
[![Backend: Go](https://img.shields.io/badge/Backend-Go%201.22+-00ADD8?logo=go)](https://go.dev)
[![Frontend: Electron](https://img.shields.io/badge/Frontend-Electron%20%7C%20React-47A248?logo=electron)](https://www.electronjs.org)
[![Powered by yt-dlp](https://img.shields.io/badge/YouTube-yt--dlp-FF0000?logo=youtube)](https://github.com/yt-dlp/yt-dlp)

Next-gen Windows download manager with parallel segmented downloading, YouTube video parsing, queue management, and Chrome extension bridge — powered by Go + Electron + yt-dlp.

> ⚠️ **Legal note:** YouTube's Terms of Service prohibit downloading videos without authorization. Use YouTube features only for content you own or have explicit permission to download.

---

## ⚡ Features

- **Segmented Downloads** — Files are split into dynamic chunks (default 8 connections) and fetched in parallel for maximum speed, like IDM.
- **Resume & Pause** — Connection lost or paused mid-download? Download state is tracked per segment and survives app restarts.
- **Queue Manager** — Set concurrent download limits, priority ordering, and automatic retries with exponential backoff.
- **Speed Limiter** — Global bandwidth cap using a token bucket algorithm to avoid choking your network connection.
- **YouTube Downloader** — Paste single videos or playlist URLs, pick quality/format, and download items smoothly. `yt-dlp` is automatically fetched on first use.
- **Browser Bridge** — Extension for Chrome & Edge that automatically intercepts downloads or lets you send them manually.
- **System Tray Integration** — Quick actions to pause all, resume all, or open the dashboard.
- **Persistent History** — Complete download state persisted in lightweight JSON under `%APPDATA%/byterush/engine`.

---

## 🛠️ Tech Stack

| Layer | Technologies |
|---|---|
| **Frontend UI** | Electron, React 18, TypeScript, Vite, CSS Modules |
| **Download Engine** | Go 1.22+ (Native multi-threaded network engine, stdlib HTTP client) |
| **Video Processing** | `yt-dlp` (Auto-downloaded binary wrapper) |
| **Browser Extension** | Manifest V3 (Chrome, Edge, Brave) |
| **Communication** | HTTP REST API + SSE (Server-Sent Events) on `127.0.0.1:29641` |

---

## 📐 Architecture

```
┌───────────────────────────────────────┐
│        Electron + React UI            │   queue list, progress, modals
└──────────────────┬────────────────────┘
                   │  HTTP + SSE (127.0.0.1:<port>)
┌──────────────────▼────────────────────┐
│          Go Download Engine           │
│  segmented downloader · resume ·      │
│  throttle · queue · yt-dlp wrapper    │
└───────────────────────────────────────┘
                   ▲
      Browser Extension (MV3) ──┘
```

The Go backend engine listens on `127.0.0.1:29641` (falls back to next available port) and exposes a JSON REST API:

| Endpoint | Purpose |
|---|---|
| `POST /api/downloads` | Add a new download `{url, filename?, folder?}` |
| `GET /api/downloads` | List all tracked downloads |
| `POST /api/downloads/{id}/pause` `/resume` `/cancel` | Control specific download state |
| `POST /api/downloads/pause-all` `/resume-all` | Bulk queue control |
| `GET/POST /api/settings` | Retrieve or update global settings |
| `POST /api/youtube/info` | Fetch formats/playlist metadata via yt-dlp |
| `POST /api/youtube/download` | Start YouTube stream download |
| `GET /api/events` | SSE stream for real-time progress and speed updates |
| `POST /api/shutdown` | Graceful shutdown signal |

---

## 📁 Repository Layout

```
byterush-download-manager/
├── apps/
│   ├── desktop/     Electron + React + TypeScript app (UI, main process, preload)
│   ├── engine/      Go download engine (single binary backend)
│   └── extension/   Chrome/Edge MV3 extension (browser integration bridge)
```

---

## 📋 Requirements

- **Operating System**: Windows 10 or Windows 11
- **For Building from Source**: Node.js 20+ & npm, Go 1.22+

---

## 🚀 Installation

### 1. Pre-built Installer (Recommended)
1. Download the latest `ByteRush Setup.exe` from [Releases](https://github.com/Shafin5714/byterush-download-manager/releases).
2. Run the executable and follow the setup instructions.
3. Launch **ByteRush** from your Start Menu or Desktop shortcut.

### 2. Building from Source
```powershell
# Clone the repository
git clone https://github.com/Shafin5714/byterush-download-manager.git
cd byterush-download-manager

# Install dependencies and build installer package
cd apps/desktop
npm install
npm run dist
```
The output installer `.exe` will be saved inside `apps/desktop/release/`.

### 3. Installing Browser Extension (Chrome / Edge / Brave)
1. Open `chrome://extensions` or `edge://extensions` in your browser.
2. Toggle on **Developer mode** in the top right corner.
3. Click **Load unpacked** and select the `apps/extension/` directory.
4. The ByteRush extension popup will confirm connection status to the desktop app.

---

## 💻 Development & Testing

### Running Desktop App in Dev Mode
```powershell
cd apps/desktop
npm install
npm run dev   # Builds Go engine, starts Vite dev server & launches Electron with hot reload
```

### Testing the Go Engine Independently
```powershell
cd apps/engine
go build -o bin/engine.exe .
powershell -ExecutionPolicy Bypass -File test.ps1   # Runs tests for downloading, pause/resume, throttling, and storage
```

---

## ❓ Troubleshooting & FAQ

<details>
<summary><b>Windows Defender / SmartScreen warning when installing</b></summary>
<br>

Because ByteRush installer is open-source and built without a paid commercial EV Code Signing Certificate, Windows Defender SmartScreen may display a standard *"Unknown Publisher"* message. Click **"More info"** and then **"Run anyway"**.
</details>

<details>
<summary><b>What if port 29641 is already in use by another app?</b></summary>
<br>

The Go engine automatically detects if port `29641` is bound and gracefully falls back to the next free port on `127.0.0.1`.
</details>

<details>
<summary><b>Where is download state & settings stored?</b></summary>
<br>

Download history and configuration are saved as JSON files in your user data directory at:
`%APPDATA%/byterush/engine/`
</details>

---

## 🤝 Contributing

Contributions, issues, and feature requests are welcome!
1. Fork the project.
2. Create your feature branch (`git checkout -b feature/AmazingFeature`).
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`).
4. Push to the branch (`git push origin feature/AmazingFeature`).
5. Open a Pull Request.

---

## 📜 License

Distributed under the MIT License. See [LICENSE](LICENSE) for more details.
