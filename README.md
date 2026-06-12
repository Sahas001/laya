# 🎵 LAYA

LAYA is a beautifully elegant, distributable **Terminal-Based Media Control Dashboard (TUI)** for Linux.

Built in **Go (Golang)** using the **Bubble Tea** ecosystem (`lipgloss`, `bubbles`), LAYA acts as a centralized controller and "now playing" dashboard for any MPRIS-compliant media player (such as `mpv`, `spotify`, `tauon`, `firefox`, `chrome`, or `vlc`). It allows you to check what is playing, adjust volume, toggle play/pause, skip tracks, and even view scrolling lyrics without ever leaving your terminal or switching windows.

---

## ✨ Features

- **🎨 Adaptive Album Art Themes**: Automatically extracts the top 3-4 dominant colors from `mpris:artUrl` (prioritizing local paths over remote URLs) using K-means clustering and updates border, text, and accent styling in real-time.
- **🎤 Synchronized Lyrics Integration**: Searches for a matching `.lrc` file in the directory of the playing track (from `xesam:url`) as a priority, falling back to the [LRCLIB](https://lrclib.net/) database. Highlights and auto-scrolls lyrics in real-time synchronized with MPRIS playback position.
- **📟 Headless status bar IPC**: Run with the `--stream` flag to output a continuous, single-line JSON format optimized for direct consumption by status bars like Waybar, with zero-overhead.
- **⚡ Auto-detection & Hot-swapping**: Scans the D-Bus session bus on startup to detect running MPRIS players. Lets you swap players on the fly.
- **📊 Real-time Interpolation**: Smoothly updates track elapsed time and progress bar locally to avoid polling D-Bus too frequently.
- **⚙️ CLI Options**: Easily list active players or start the TUI directly connected to specific players via flags.

---

## 🎹 Keybindings & Navigation

LAYA supports robust, native Vim keybindings throughout the application:

### 📱 Main Dashboard View
| Key | Action |
| --- | --- |
| `[Space]` | Play / Pause |
| `[n]` / `[Right Arrow]` | Next Track |
| `[p]` / `[Left Arrow]` | Previous Track |
| `[` | Seek backward 5 seconds |
| `]` | Seek forward 5 seconds |
| `[l]` | Open/Close Lyrics View |
| `[H]` | Cycle active player backward |
| `[L]` | Cycle active player forward |
| `[↑]` / `[+]` | Volume Up (+5%) |
| `[↓]` / `[-]` | Volume Down (-5%) |
| `[s]` | Switch / Select Media Player (View Player Selection) |
| `[q]` / `[Ctrl+C]` | Quit Application |

### 🎤 Lyrics View
| Key | Action |
| --- | --- |
| `[j]` / `[↓]` | Scroll Down |
| `[k]` / `[↑]` | Scroll Up |
| `[Space]` | Play / Pause |
| `[n]` / `[Right Arrow]` | Next Track |
| `[p]` / `[Left Arrow]` | Previous Track |
| `[` | Seek backward 5 seconds |
| `]` | Seek forward 5 seconds |
| `[H]` | Cycle active player backward |
| `[L]` | Cycle active player forward |
| `[+]` / `[=]` | Volume Up (+5%) |
| `[-]` | Volume Down (-5%) |
| `[l]` / `[Esc]` | Close Lyrics View / Return to Dashboard |
| `[q]` / `[Ctrl+C]` | Quit Application |

### 👥 Player Selection View
| Key | Action |
| --- | --- |
| `[k]` / `[Up Arrow]` | Select Previous Player |
| `[j]` / `[Down Arrow]` | Select Next Player |
| `[Enter]` | Connect to Selected Player |
| `[r]` | Refresh Active Players list |
| `[Esc]` | Cancel / Return to Dashboard (if a player was connected) |

---

## 📟 Headless Status Bar Streaming (Waybar IPC)

LAYA can be run headlessly using the `--stream` flag. In this mode, LAYA does not draw any UI. Instead, it hooks onto the active player (automatically choosing the one currently playing) and continuously outputs a single-line JSON structure on stdout every 1 second, with zero-overhead:

```json
{"artist":"Daft Punk","title":"Instant Crush","status":"Playing","progress_bar_chars":"████░░░░░░","text":"Daft Punk - Instant Crush","alt":"playing","tooltip":"Title: Instant Crush\nArtist: Daft Punk\n...","class":"playing","percentage":40}
```

### Waybar Custom Module Configuration

You can easily integrate LAYA into your [Waybar](https://github.com/Alexays/Waybar) configuration:

```jsonc
"custom/laya": {
    "format": "{icon} {}",
    "return-type": "json",
    "escape": true,
    "exec": "laya-tui --stream",
    "on-click": "laya-tui",
    "format-icons": {
        "playing": "",
        "paused": "",
        "stopped": ""
    }
}
```

Add `"custom/laya"` to your `modules-left`, `modules-center`, or `modules-right` array.

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
  -stream
    	Continuously stream JSON metadata to stdout for status bars (e.g. Waybar)
  -version
    	Print version information and exit
```
