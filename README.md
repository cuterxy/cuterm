# cuterm

English | **[简体中文](README.zh-CN.md)**

A shared terminal server: start a single binary, open the management page in your browser, and create, attach to, and close terminals. A running terminal can be attached by multiple browser pages at once — all clients share input, output, and the recent scrollback (128 KB ring buffer) in real time.

## Features

- Open `http://localhost:7681` in a browser for the app page (multi-terminal management + terminal use)
- Separate settings page at `http://localhost:7681/config.html`: listen port, shell for new terminals, UI language, and terminal font, font size, and color scheme
- Bilingual UI and tray menu, English and 简体中文 (switch on the settings page — applies instantly and syncs to the tray menu; follows the browser / system language by default)
- Daemonizes itself on start and shows a system tray icon (menu bar on macOS)
- Tray menu: open app page, open settings page, quit the service
- New terminals use the login shell (`$SHELL` on Unix, PowerShell / cmd on Windows)
- Attach to any running terminal; exited terminals can still be attached to view their scrollback
- Close running terminals
- Multiple clients attached to the same terminal: input, output, resize, and scrollback sync in real time
- Single-binary distribution with the frontend (xterm.js) fully embedded — works offline

## Installation

One-line install on macOS / Linux (detects OS and architecture, downloads the latest release):

```bash
curl -fsSL https://raw.githubusercontent.com/cuterxy/cuterm/main/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/cuterxy/cuterm/main/install.ps1 | iex
```

You can also download packages manually from [Releases](https://github.com/cuterxy/cuterm/releases): a macOS `.pkg` installer (installs `cuterm.app` with its icon to /Applications, plus the `cuterm` CLI to /usr/local/bin), Linux `.deb` / `.rpm` packages (they pull in libayatana-appindicator3 automatically and add a cuterm launcher with icon), a Windows `setup.exe` installer (carries the cuterm icon, adds cuterm to your PATH, uninstallable from "Apps & Features"), or plain archives. Go users can run `go install github.com/cuterxy/cuterm@latest` (requires a C toolchain).

Notes:

- Linux requires libayatana-appindicator3 at runtime (Debian/Ubuntu: `sudo apt install libayatana-appindicator3-1` — installed automatically by the `.deb` / `.rpm` packages)
- On macOS, if a browser-downloaded binary or `.pkg` is blocked by Gatekeeper (the downloads are not signed with an Apple certificate), right-click → Open, or run `xattr -d com.apple.quarantine cuterm` (installing with the curl script above avoids this)
- Windows requires Windows 10 1809 or newer (depends on ConPTY); the unsigned installer may trigger a SmartScreen prompt — choose "More info" → "Run anyway"

## Build

The system tray requires CGO, so cross-compilation is not supported — build natively on each target OS (releases are built natively on per-platform CI runners, see `.github/workflows/release.yml`; pushing a `v*` tag publishes automatically):

```bash
go build -o cuterm .        # native build (CGO required)
go build -o cuterm-hub ./cmd/cuterm-hub   # the cuterm-hub companion app
./build.sh 1.0.0            # build both apps' current-platform releases into dist/
```

Requirements:

- macOS / Linux: a C toolchain (clang / gcc)
- Linux: libayatana-appindicator3 development headers to build (Debian/Ubuntu: `sudo apt install libayatana-appindicator3-dev`)
- Windows: releases are built with `-H windowsgui` so no console window pops up

> Note: binaries linked with older Go toolchains (e.g. 1.22) on newer macOS may miss `LC_UUID` and fail to launch — upgrade your Go toolchain.

## Usage

```bash
./cuterm                  # daemonize, tray icon appears, listens on :7681
./cuterm -addr :9000      # custom port
./cuterm -foreground      # run in the foreground (debug; logs go to the terminal)
./cuterm -version         # print version
```

After startup, use the tray menu "Open App" to enter `http://localhost:7681` (or open it manually in a browser). "Open Settings" lets you change the listen port, the shell for new terminals, the UI language, and the terminal font, font size, and color scheme (the app page sidebar also has links to the settings page and a language toggle). Choose "Quit" in the tray menu to stop the service, or simply `kill` the background process.

Changes on the settings page take effect immediately and persist to `~/.cuterm/config.json`, applied automatically on the next start; an explicit `-addr` flag takes precedence over the configured port. In daemon mode logs go to `~/.cuterm/cuterm.log`.

## cuterm-hub

cuterm-hub is the companion fleet manager: it connects to any number of cuterm instances ("nodes") and puts all their terminals on one page. The hub proxies REST and WebSocket traffic transparently — the nodes run stock cuterm, no agent or plugin needed.

- App page at `http://localhost:7682`: terminals grouped by node; create, attach to, rename, and close terminals on any node
- Settings page at `http://localhost:7682/config.html`: add/remove nodes (name + `host:port`), pick each node's shell, plus the listen port, UI language, launch at login, and terminal font, font size, color scheme, and scrollback lines
- Same experience as cuterm: daemonizes on start, system tray icon (blue), bilingual UI, settings persisted to `~/.cuterm-hub/config.json`

Install with the same scripts by naming the app:

```bash
curl -fsSL https://raw.githubusercontent.com/cuterxy/cuterm/main/install.sh | sh -s cuterm-hub
```

```powershell
.\install.ps1 -App cuterm-hub
```

Releases publish cuterm-hub too (same artifact names with the `cuterm-hub-` prefix: archives, `.pkg`, `.deb` / `.rpm`, `setup.exe`). Build from source:

```bash
go build -o cuterm-hub ./cmd/cuterm-hub
```

### Hub HTTP API

The hub serves the same settings endpoints as cuterm (`/api/port`, `/api/appearance`, `/api/language`, `/api/autostart`, `/api/version`) and adds:

| Method | Path | Description |
|---|---|---|
| GET | `/api/nodes` | List nodes with live status: `[{"id":"...","name":"...","addr":"host:7681","online":true,"version":"..."}]` |
| POST | `/api/nodes` | Add a node, body: `{"name":"...","addr":"host:7681"}` (name optional; port defaults to 7681) |
| PATCH | `/api/nodes/{id}` | Edit a node's name / address |
| DELETE | `/api/nodes/{id}` | Remove a node |
| GET/POST/PATCH/DELETE | `/api/nodes/{id}/terminals...` | Proxied verbatim to the node's own `/api/terminals...` |
| GET/POST | `/api/nodes/{id}/shells`, `/api/nodes/{id}/shell` | Proxied to the node's shell settings |
| GET | `/ws/nodes/{id}/terminals/{tid}` | WebSocket attach, bridged to the node |

## Architecture

- `internal/terminal` — terminal session management: one PTY per terminal, output fanned out to all subscribers, 128 KB of scrollback kept for new clients to replay; two platform implementations, `pty_unix.go` (creack/pty) and `pty_windows.go` (ConPTY)
- `internal/server` — HTTP API + WebSocket; WebSocket uses binary frames whose first byte is the type: `0` output, `1` input, `2` resize, `3` terminal exited
- `web/` — the app page (`index.html`: terminal management + xterm.js terminal use) and the settings page (`config.html`: port, shell, font, font size, color scheme); font and theme presets live in `web/themes.js`, UI strings in English and Chinese in `web/i18n.js`; everything is embedded into the binary with `go:embed`
- `internal/hub` — cuterm-hub's proxy server: node registry with live status, transparent REST proxying to the nodes' APIs, and a WebSocket bridge to the nodes' terminal sockets
- `cmd/cuterm-hub` — the cuterm-hub binary: same shape as the cuterm main package (config, daemon, tray), with its own embedded web UI in `cmd/cuterm-hub/web/` and blue icon assets in `cmd/cuterm-hub/assets/`
- The tray menu language follows the language set on the settings page and switches instantly; when unset it is chosen from the system language (`lang_unix.go` / `lang_windows.go`)

### HTTP API

| Method | Path | Description |
|---|---|---|
| GET | `/api/terminals` | List all terminals |
| POST | `/api/terminals` | Create a terminal, body: `{"name":"...", "cols":80, "rows":24}` (all optional, name auto-generated if empty) |
| PATCH | `/api/terminals/{id}` | Rename a terminal, body: `{"name":"new name"}` |
| DELETE | `/api/terminals/{id}` | Close a terminal |
| GET | `/ws/terminals/{id}` | WebSocket attach to a terminal |
| POST | `/api/port` | Change the listen port, body: `{"port":9000}` (editable on the settings page) |
| GET | `/api/shells` | Current shell and choices: `{"current":"/bin/zsh","available":[...]}` |
| POST | `/api/shell` | Set the shell for new terminals, body: `{"shell":"/bin/zsh"}` (empty string restores auto-detection; applies to new terminals only) |
| GET | `/api/appearance` | Current appearance config: `{"fontFamily":"...","fontSize":14,"theme":"default"}` (missing fields mean built-in defaults) |
| POST | `/api/appearance` | Set terminal font, font size, color scheme, body: `{"fontFamily":"...","fontSize":14,"theme":"dracula"}` |
| GET | `/api/language` | Current UI language: `{"language":"zh-CN"}` (empty string means follow the browser / system language) |
| POST | `/api/language` | Set the UI language, body: `{"language":"en"}` (`"en"` / `"zh-CN"` / `""`; switches the tray menu language instantly) |

## Security Warning

This program has no authentication — anyone who can reach the port gets a shell. Only use it on trusted networks / localhost, or put a reverse proxy with authentication in front of it.

## License

[MIT License](LICENSE)
