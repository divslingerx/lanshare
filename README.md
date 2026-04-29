# filehub

A cross-platform desktop app for real-time LAN file sharing. Add folders, discover peers on your local network automatically, and keep files in sync — no cloud, no accounts, no configuration servers.

## How it works

- **Watch folders** are monitored for changes. When a file changes, peers who have subscribed to that folder receive it automatically over a direct HTTP connection.
- **Shared folders** are browsable by peers on demand (read-only, pull-based).
- Peers are discovered via mDNS (Bonjour/Zeroconf) — no IP addresses to configure.
- Changes stream over SSE (Server-Sent Events). Offline changes are detected on startup and synced when a peer reconnects.

## Features

- Zero-config peer discovery over mDNS (`_filehub._tcp`)
- Real-time file sync using Server-Sent Events
- Catch-up sync on reconnect — no missed changes
- Watch mode (push) and Shared mode (pull) per folder
- Atomic file writes — no partial files on disk
- xxHash64 change detection (fast, collision-resistant)
- Per-subscription configurable destination paths
- Offline change detection on startup
- Port, display name, and base storage path configurable in-app

## Stack

- **Go 1.23** + **Wails v2** — native desktop app with a web frontend
- **React 18** + **Vite** — frontend
- **mDNS** via `grandcat/zeroconf`
- **fsnotify** with 500 ms debounce for file watching
- **xxHash** via `cespare/xxhash`

## Development

Prerequisites: Go 1.23+, Node.js 18+, [Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Run in dev mode (hot reload)
wails dev
```

The Wails dev server starts on `http://localhost:34115` — open it in a browser to use devtools against the Go backend.

## Building

```bash
wails build
```

The binary is written to `build/bin/`.

## Testing

```bash
go test ./... -race
```

All packages include tests. The `-race` flag is recommended — the codebase uses goroutines for file watching, SSE streaming, and peer discovery.

## Configuration

Config is stored in the OS user config directory (`%APPDATA%\filehub\config.json` on Windows, `~/.config/filehub/config.json` on Linux, `~/Library/Application Support/filehub/config.json` on macOS).

File manifests (used for change detection) are stored alongside the config, keyed by folder ID.

## Default port

`47990` — configurable in Settings.

## Project structure

```
config/      Config schema, load/save, folder ID hashing
discovery/   mDNS advertiser and browser (grandcat/zeroconf)
manifest/    Per-folder file manifest with xxHash entries
server/      HTTP server: file serving, SSE hub, /since endpoint
transfer/    HTTP client: pull individual files and catch-up sync
watcher/     fsnotify watcher with debounce and manifest updates
frontend/    React UI (Vite)
app.go       Wails app bindings — wires all packages together
main.go      Entry point
```
