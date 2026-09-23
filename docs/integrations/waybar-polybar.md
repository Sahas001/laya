# Waybar & Polybar Integration Guide

LAYA supports a headless streaming mode via the `--stream` flag. When enabled, it outputs single-line JSON events over stdout whenever playback state, current track, artist, album art, or position updates. This is designed for direct consumption by Linux status bars such as **Waybar** and **Polybar**.

## 1. Waybar Configuration

Add a custom module to your Waybar configuration (`~/.config/waybar/config.jsonc`):

```jsonc
"custom/laya": {
    "format": "{icon} {}",
    "return-type": "json",
    "max-length": 45,
    "exec": "laya --stream",
    "on-click": "playerctl play-pause",
    "on-click-right": "playerctl next",
    "on-click-middle": "playerctl previous",
    "format-icons": {
        "Playing": "\udb80\udfdf",
        "Paused": "\udb80\udfe4",
        "Stopped": "\udb80\udfe1"
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

#custom-laya.Playing {
    color: #a6e3a1;
}

#custom-laya.Paused {
    color: #fab387;
}
```

## 2. Polybar Configuration

Add a custom IPC script module in `~/.config/polybar/config.ini`:

```ini
[module/laya]
type = custom/script
exec = laya --stream
tail = true
format = <label>
label = %output%
click-left = playerctl play-pause
click-right = playerctl next
```

## 3. Testing the Stream

You can verify the stream directly from your terminal:

```bash
laya --stream | jq .
```

Each payload provides `text`, `alt`, `tooltip`, and `class` fields conforming to Waybar JSON specs.
