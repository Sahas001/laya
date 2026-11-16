# Waybar & Polybar Integration Guide

LAYA supports a headless streaming mode via the `--stream` flag. It prints one line to stdout whenever playback state, track metadata or position changes: immediately on MPRIS signals, and otherwise every `--interval` (default `1s`). Identical consecutive lines are suppressed.

- By default each line is a **JSON** object for **Waybar**.
- With `--format "<template>"` each line is **plain text** for **Polybar**, i3blocks, tmux and similar tools.

When no `--player` is given, LAYA follows whichever player is currently playing and stays on the last one when everything is paused.

## 1. Waybar Configuration

Add a custom module to your Waybar configuration (`~/.config/waybar/config.jsonc`):

```jsonc
"custom/laya": {
    "format": "{icon} {}",
    "return-type": "json",
    "max-length": 45,
    "exec": "laya-tui --stream",
    "on-click": "playerctl play-pause",
    "on-click-right": "playerctl next",
    "on-click-middle": "playerctl previous",
    "format-icons": {
        "playing": "\udb80\udfdf",
        "paused": "\udb80\udfe4",
        "stopped": "\udb80\udfe1"
    }
}
```

And add styling in `~/.config/waybar/style.css`:

```css
#custom-laya {
    padding: 0 10px;
    margin: 0 4px;
    background-color: #1e1e2e;
    color: #cdd6f4;
    border-radius: 8px;
}

#custom-laya.playing {
    color: #a6e3a1;
}

#custom-laya.paused {
    color: #fab387;
}
```

## 2. Polybar Configuration

Polybar displays script output verbatim, so use `--format` to emit plain text instead of JSON. Add a custom script module in `~/.config/polybar/config.ini`:

```ini
[module/laya]
type = custom/script
exec = laya-tui --stream --format "{icon} {artist} - {title}"
tail = true
format = <label>
label = %output%
click-left = playerctl play-pause
click-right = playerctl next
```

## 3. Testing the Stream

You can verify the stream directly from your terminal:

```bash
laya-tui --stream | jq .
```

Each JSON payload provides the Waybar fields `text`, `alt`, `tooltip`, `class` and `percentage`, plus `artist`, `title`, `album`, `player`, `status`, `position`, `length` and `progress_bar_chars` for custom scripts.

### `--format` placeholders

| Placeholder | Example |
| --- | --- |
| `{artist}` `{title}` `{album}` | `Daft Punk`, `Contact`, `Random Access Memories` |
| `{player}` | `Spotify` |
| `{status}` / `{icon}` | `Playing` / `▶` (`⏸`, `⏹`) |
| `{position}` / `{length}` | `01:23` / `06:21` (`live` for streams) |
| `{percentage}` / `{bar}` | `21` / `██░░░░░░░░` |
| `{text}` | the same text Waybar shows |

If a track has no artist (common for browser videos), separators such as `{artist} - ` are dropped automatically.

```bash
laya-tui --stream --format "{icon} {title} [{position}/{length}]"
```
