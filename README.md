# 🎵 Laya TUI

`laya-tui` is a beautifully elegant, distributable **Terminal-Based Media Control Dashboard (TUI)** for Linux. 

Built in **Go (Golang)** using the **Bubble Tea** ecosystem (`lipgloss`, `bubbles`), `laya-tui` acts as a centralized controller and "now playing" dashboard for any MPRIS-compliant media player (such as `mpv`, `spotify`, `tauon`, `firefox`, `chrome`, or `vlc`). It allows you to check what is playing, adjust volume, toggle play/pause, skip tracks, and even view scrolling lyrics without ever leaving your terminal or switching windows.

---

## ✨ Features

- **⚡ Auto-detection & Hot-swapping**: Scans the D-Bus session bus on startup to detect running MPRIS players. Lets you swap players on the fly.
- **🎨 Catppuccin-Inspired Aesthetics**: Uses a soft, minimalist, dark-mode styling with subtle pastel highlights.
- **📊 Real-time Interpolation**: Smoothly updates track elapsed time and progress bar locally to avoid polling D-Bus too frequently.
- **🎤 Synchronized Lyrics Integration**: Hits the open-source [LRCLIB](https://lrclib.net/) database to fetch plain/synced lyrics for the currently playing track, rendering them inside an elegant scrollable panel.
- **⚙️ CLI Options**: Easily list active players or start the TUI directly connected to a specific player via flags.

---

## 🎹 Keybindings

### 📱 Main Dashboard View
| Key | Action |
| --- | --- |
| `[Space]` | Play / Pause |
| `[n]` / `[Right Arrow]` | Next Track |
| `[p]` / `[Left Arrow]` | Previous Track |
| `[↑]` / `[k]` / `[+]` | Volume Up (+5%) |
| `[↓]` / `[j]` / `[-]` | Volume Down (-5%) |
| `[l]` | Open/Close Lyrics View |
| `[s]` | Switch / Select Media Player |
| `[q]` / `[Ctrl+C]` | Quit Application |

### 🎤 Lyrics View
| Key | Action |
| --- | --- |
| `[j]` / `[↓]` | Scroll Down |
| `[k]` / `[↑]` | Scroll Up |
| `[Space]` | Play / Pause |
| `[n]` / `[Right Arrow]` | Next Track |
| `[p]` / `[Left Arrow]` | Previous Track |
| `[+]` / `[=]` | Volume Up (+5%) |
| `[-]` | Volume Down (-5%) |
| `[l]` / `[Esc]` | Close Lyrics View |
| `[q]` / `[Ctrl+C]` | Quit Application |

---

## 🚀 Installation

### Prerequisites
- A Linux environment running D-Bus.
- Go 1.22 or higher.

### Quick Install
To install the latest version directly to your Go path (`$GOPATH/bin`):
```bash
go install github.com/Sahas001/laya-tui@latest
```

Ensure `$GOPATH/bin` is in your environment shell path (e.g. `export PATH=$PATH:$GOPATH/bin`).

### From Source
1. Clone the repository:
   ```bash
   git clone https://github.com/Sahas001/laya-tui.git
   cd laya-tui
   ```
2. Build the project using the included `Makefile`:
   ```bash
   make build
   ```
3. Run the compiled binary:
   ```bash
   ./laya-tui
   ```
4. Or install it locally:
   ```bash
   make install
   ```

---

## 🛠️ Command-Line Flags

```bash
Laya TUI - Distributable MPRIS Media Control Dashboard

Usage:
  laya-tui [flags]

Flags:
  -list
    	List active MPRIS media players and exit
  -player string
    	Directly connect to a player by name (case-insensitive substring match)
  -version
    	Print version information and exit
```

---

## 🏗️ Project Architecture

```
laya-tui/
├── Makefile             # Build automation
├── README.md            # Project documentation
├── go.mod               # Dependencies
├── go.sum               # Dependency hashes
├── main.go              # App entrypoint
├── app/
│   └── app.go           # CLI flag parsing & Bubble Tea coordinator
├── mpris/
│   └── mpris.go         # Direct D-Bus MPRIS wrapper (no heavy external library)
└── ui/
    ├── styles.go        # Lipgloss color palettes & layouts
    ├── view.go          # Core TUI state & Bubble Tea model
    ├── dashboard.go     # Dashboard/Now Playing render
    ├── player_select.go # Player list & selection UI
    └── lyrics.go        # LRCLIB API client & parser
```

- **`mpris`**: Interacts directly with `/org/mpris/MediaPlayer2` using `github.com/godbus/dbus/v5`. Fetches player properties like volume, playback status, and track metadata (title, artist, album, duration).
- **`ui`**: Implements the Bubble Tea model. Performs smooth progress bar interpolation and orchestrates navigation between player selection and media controller views.
- **`lyrics`**: Asynchronously hits `https://lrclib.net/api/search` with the currently playing track's metadata, falling back to stripping LRC timestamp tags if only synced lyrics exist.
