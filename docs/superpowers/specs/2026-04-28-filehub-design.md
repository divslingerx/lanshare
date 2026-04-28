# filehub — Design Spec

**Date:** 2026-04-28  
**Stack:** Go 1.23, Wails v2, React 18 + Vite  
**Target platforms:** Windows, macOS, Linux

---

## Overview

filehub is a desktop app for sharing files between computers on the same local network. Users add local folders, set them to Watch or Shared mode, and other filehub instances on the LAN discover each other automatically. No cloud, no accounts, no configuration beyond opening the app.

---

## Architecture

Each filehub instance runs a Go HTTP server in the background on port **47990**. mDNS (Zeroconf) advertises the device on the LAN so peers are discovered without manual IP entry. Change notifications are pushed over SSE (Server-Sent Events) — subscribers hold a persistent connection open to each peer they follow. File transfers are plain HTTP GETs. The React frontend communicates with the Go backend exclusively through Wails bindings; it never speaks HTTP directly.

```
[Peer A - filehub]                    [Peer B - filehub]
  HTTP server :47990  <── GET file ──  HTTP client
  SSE endpoint        ── change evt ─> SSE subscriber
  mDNS advertise      <── discover ──  mDNS browse
  Wails backend       <── bindings ──  React frontend
```

Port 47990 is configurable in Settings in case of conflict.

---

## Device Identity

- **ID:** system hostname — used internally for mDNS advertisement, peer tracking, and storage subfolder naming. Never changes.
- **Display name:** user-set label, defaults to hostname. Shown in the UI peer list and on remote peers' UIs. Purely cosmetic — renaming it has no structural effect.

Config stores both. Peers advertise display name over mDNS but are keyed by hostname internally.

---

## Folder ID

Each folder is assigned a stable `folder_id` when added — the first 8 hex characters of the xxHash64 of its absolute path (e.g. `a3f1bc92`). Stored in config alongside the path. Used in HTTP API routes and SSE events so URLs remain stable even if the display path changes. Generated once, never regenerated.

---

## Folder Model

Two distinct local folder types:

### Watch
- Monitored by `fsnotify` for file system changes.
- Maintains a **file manifest**: `map[relativePath]{mtime, size, xxhash64}`.
- On change: debounce 500ms (fsnotify fires multiple events per save) → update manifest → broadcast change event over all open SSE connections.
- Subscribers can pull changed files on demand.

### Shared
- No file watching, no manifest.
- Serves files via HTTP on request.
- Peers browse the directory listing and download individual files manually.

---

## Subscription Model

A **subscription** is an opt-in to a remote peer's Watch folder. It stores:

```json
{
  "peer_hostname": "taylors-macbook-air-2026",
  "remote_folder": "/Users/taylor/Documents/Projects",
  "local_dest": "~/filehub/taylors-macbook-air-2026/Projects",
  "last_synced_at": "2026-04-28T10:32:00Z"
}
```

- `local_dest` defaults to `~/filehub/{peer_hostname}/{folder_name}/` but can be overridden per subscription to any local path (e.g. an external drive).
- `last_synced_at` is updated after every successful sync and used for catch-up on reconnect.

Only Watch folders can be subscribed to. Shared folders are browse-and-download only.

---

## Data Flow

### Change broadcast (outbound)
1. `fsnotify` detects a file event in a watched folder.
2. Go debounces within a 500ms window (rapid multi-event saves collapse to one).
3. Manifest is updated with new `{mtime, size, xxhash64}`.
4. Change event pushed to all subscribers over SSE:
   ```json
   { "folder_id": "a3f1bc92", "changes": [{"path": "relative/file.txt", "op": "write|delete", "mtime": 1714299120, "size": 204800}] }
   ```

### File pull (inbound subscription)
1. Subscriber receives SSE change event.
2. Compares changed file list against local manifest.
3. Issues HTTP GET to `http://{peer}:47990/files/{folder_id}/{relative_path}` for each diffed file.
4. Writes to `local_dest`, updates local manifest, updates `last_synced_at`.

### Startup catch-up
1. Load config, start HTTP server, start mDNS.
2. For each known peer hostname, wait for mDNS to resolve it (or timeout after 10s and mark offline).
3. For each subscription to an online peer, call `GET /since?folder={id}&t={last_synced_at}`.
4. Peer returns the same `changes` array format as SSE events; subscriber pulls the diff.
5. For each local watched folder, stat-walk and compare against saved manifest — flag any changes that occurred while the app was closed (displayed in UI as "X files changed while offline").

### Manual browse & download (shared folders)
- `GET /browse/{folder_id}/` returns a JSON directory listing.
- `GET /files/{folder_id}/{relative_path}` serves the file.
- No SSE, no manifest, no subscription required.

---

## Change Detection

Two-stage strategy to minimise I/O:

1. **Fast pass — mtime + size:** a single `stat()` call per file. If both match the manifest, file is considered unchanged. No hashing. Handles ~99% of cases.
2. **Confirmation — xxHash64:** run only on files where mtime or size differs. Uses `github.com/cespare/xxhash` (~10 GB/s). Updates manifest entry on change.

Manifest schema per file:
```json
{ "mtime": 1714299120, "size": 204800, "xxhash": "a3f1bc920d4e7c12" }
```

---

## Config Persistence

Single JSON file written via `os.UserConfigDir()`:

| OS      | Path |
|---------|------|
| Windows | `%APPDATA%\filehub\config.json` |
| macOS   | `~/Library/Application Support/filehub/config.json` |
| Linux   | `~/.config/filehub/config.json` |

Config schema:
```json
{
  "device_id": "taylors-macbook-air-2026",
  "display_name": "macbook",
  "port": 47990,
  "base_storage": "~/filehub",
  "folders": [
    { "id": "a3f1bc92", "path": "/Users/taylor/Documents/Projects", "mode": "watch" }
  ],
  "subscriptions": [
    {
      "peer_hostname": "DESKTOP-WIN11",
      "remote_folder": "C:\\Users\\asd\\Desktop\\Assets",
      "local_dest": "",  // empty = use default ~/filehub/{peer_hostname}/{folder_name}/
      "last_synced_at": "2026-04-28T10:32:00Z"
    }
  ]
}
```

Config is written to disk on every mutation (add/remove folder, change mode, update display name). No explicit save button.

---

## Storage Layout

```
~/filehub/
  DESKTOP-WIN11/
    Assets/          ← subscription mirror
  taylors-macbook-air-2026/
    Projects/        ← subscription mirror
```

Storage subfolders are keyed by peer **hostname** (not display name) so renaming a device doesn't break existing synced files.

---

## Cross-Platform Considerations

| Concern | Approach |
|---------|----------|
| Path separators | `filepath.Join()` everywhere, never hardcode `/` or `\` |
| Home directory | `os.UserHomeDir()` |
| Config directory | `os.UserConfigDir()` |
| File watching | `github.com/fsnotify/fsnotify` (Windows/macOS/Linux) |
| mDNS discovery | `github.com/grandcat/zeroconf` (Bonjour/Avahi/native) |
| Case sensitivity | Paths compared case-insensitively on Windows/macOS, case-sensitively on Linux |
| Symlinks | Skipped during manifest walks (Windows requires elevated perms) |
| Drive letters | Stored and displayed as-is; remote peers see the full Windows path but only need it for display |

---

## Error Handling

- **Peer drops mid-transfer:** file is discarded, no partial write committed. Retried on next reconnect using `last_synced_at`.
- **Watched folder deleted/moved:** marked as missing in UI with a warning badge. Not removed from config automatically.
- **Permission errors on file read:** logged, file skipped. Shown as a warning count in the UI.
- **Port conflict:** startup fails with a clear error dialog prompting the user to change the port in Settings.
- **Peer offline at startup:** subscription queued, retried when peer re-appears via mDNS. No blocking.

---

## HTTP API (internal, port 47990)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/peers` | Returns this device's identity + folder list |
| `GET` | `/browse/{folder_id}/` | Directory listing for a Shared folder |
| `GET` | `/files/{folder_id}/{path}` | Serve a file (Watch or Shared) |
| `GET` | `/since?folder={id}&t={iso8601}` | Files changed since timestamp (Watch folders) |
| `GET` | `/events/{folder_id}` | SSE stream for change events (Watch folders) |

---

## UI Summary

Three-panel layout (per mockup):
- **Left sidebar:** nav (My Folders / Network / Settings), discovered peers with live pulse indicator, this device's IP + port.
- **Main panel:** folder list — each card shows path, file count, size, mode toggle (Watch / Shared), remove button.
- **Right panel:** active transfers with progress bars, completed transfer history.
- **Add Folder modal:** path input + Browse button + mode selector (Watch / Shared).

Display name is editable in Settings. Port and base storage path are also configurable there.

---

## Out of Scope (v1)

- Authentication / pairing (LAN = trusted; passwords can be added in v2)
- Conflict resolution for simultaneous edits to the same file
- Resumable / chunked transfers
- Mobile clients
- Relay / NAT traversal (LAN only)
