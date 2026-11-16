# LAYA v1.1 Upgrade Plan

Audit of `v1.0.0` (commit `5d89326`) and the changes made on branch `upgrade/v1.1`.
Items are grouped by priority; every item marked ✅ is implemented on this branch.

## 1. Packaging & repository hygiene

| # | Issue | Fix |
|---|-------|-----|
| ✅ 1.1 | Module path is `github.com/Sahas001/laya-tui` but the repo is `laya`, so the README's `go install github.com/Sahas001/laya@latest` fails. | Module renamed to `github.com/Sahas001/laya`; entrypoint moved to `cmd/laya-tui` so the binary keeps its `laya-tui` name. |
| ✅ 1.2 | A 12 MB compiled `laya-tui` binary is committed despite being in `.gitignore`. | Removed from the index (kept ignored). |
| ✅ 1.3 | `Version` is a hard-coded constant. | Injected with `-ldflags` from `git describe`; falls back to Go module build info for `go install`. |
| ✅ 1.4 | Code is not `gofmt`-ed; `go vet` warns about `Seek(int64) error` clashing with `io.Seeker`. | `gofmt` applied; method renamed to `SeekBy`. |
| ✅ 1.5 | README says Go 1.22+, `go.mod` demands 1.26.4; build-from-source says `cd laya-tui`. | `go` directive lowered to 1.24 (lowest the deps allow); docs fixed. |
| ✅ 1.6 | Dependencies slightly stale. | Patch/minor bumps (`godbus`, `x/ansi`, …). Bubble Tea v2 migration left for later (breaking API). |
| ✅ 1.7 | Makefile is minimal. | Adds `VERSION` ldflags, `fmt`, `vet`, `check`, `install` to `~/.local/bin`. |

## 2. Logic bugs

| # | Issue | Fix |
|---|-------|-----|
| ✅ 2.1 | All D-Bus calls run **synchronously inside `Update`**; a hung player freezes the UI for up to 25 s (D-Bus default timeout). | State fetches and player commands run as async `tea.Cmd`s with a 2 s context timeout. Stale replies from a previous player are dropped. |
| ✅ 2.2 | `NewModel` opens a second session-bus connection. | The connection from `app.Run` is injected. |
| ✅ 2.3 | The theme only refreshes when title/artist change, so art that arrives after the metadata (common with Spotify/browsers) or changes on its own is never picked up. | Theme is keyed on the art URL itself; lyrics are keyed on artist + title (case-insensitive), so replaying a track doesn't refetch them. |
| ✅ 2.4 | Lyrics viewport width is `terminal width - 8`, wider than the 50-column box, so long lines wrap into a garbled layout. | Single layout function computes box/content widths; viewport matches content width and long lyric lines are word-wrapped, with auto-scroll tracking wrapped row offsets. |
| ✅ 2.5 | Plain lyrics written to `<track>.lrc` are later read back as *synced*, parse to zero lines, and show "No lyrics text available". | Synced parse falling back to plain; lyrics cache stores JSON with both variants. |
| ✅ 2.6 | Fetched lyrics are written **next to the user's music files** (surprising, fails on read-only libraries, never caches streaming players like Spotify). | Cache moved to `$XDG_CACHE_HOME/laya/lyrics/`, keyed by artist/title/duration; works for every player. Existing sidecar `.lrc` files are still read first. |
| ✅ 2.7 | `file://` URLs decoded with `QueryUnescape`, turning `+` into spaces (breaks `AC+DC/…` paths). | Decoded via `url.Parse`. Same fix for album art. |
| ✅ 2.8 | LRCLIB stage-1 leaks the response body on non-200; stage-2 blindly takes `results[0]`. | Body always closed; best result chosen by synced-lyrics availability and closest duration. |
| ✅ 2.9 | Position interpolation ignores `Rate` (1.5× playback drifts). | `Rate` read and applied in TUI and stream mode. |
| ✅ 2.10 | Synced-lyric highlight only updates while playing; after seeking while paused it is stale. | Lyrics view refreshes on every state update. |
| ✅ 2.11 | Album-art colour extraction can finish out of order (fast skipping shows the wrong theme), and a track without art keeps the previous track's theme. | Theme messages tagged with art URL; tracks without art reset to the default palette. |
| ✅ 2.12 | Visualizer animation frame wraps at 24 (`24 × 0.8` rad isn't a multiple of 2π) → visible jump every 2.4 s. | Continuous phase counter. |
| ✅ 2.13 | Stream mode: with `--player`, an erroring player is returned without validation → **busy loop** with no sleep. Without `--player`, it sticks to the first paused player even after another starts playing. | Candidates are validated; selection is re-evaluated whenever the current player isn't playing. |
| ✅ 2.14 | `mpris:length`/`Position` assumed `int64`; some players send `uint64`/`int32`/`double` → duration shows `--:--`. | Numeric variants normalised. |
| ✅ 2.15 | Errors from player commands (e.g. seek on a non-seekable stream) are silently dropped; `m.err` is set but never displayed. | Transient toast line shows errors and capability warnings. |
| ✅ 2.16 | Polybar guide uses `--stream` raw JSON, which Polybar prints verbatim. | New `--format` flag emits plain text from a template (`{artist} - {title}`); guide updated. |
| ✅ 2.17 | `playerctld` appears as a separate "player" in cycling, duplicating the real one. | `H`/`L` cycling skips `playerctld`; selection list labels it clearly. |

## 3. Responsiveness

| # | Change |
|---|--------|
| ✅ 3.1 | Subscribe to MPRIS `PropertiesChanged`/`Seeked` and `NameOwnerChanged` signals → instant UI updates on play/pause/skip and players appearing/disappearing (1 s polling kept as fallback). |
| ✅ 3.2 | Stream mode prints immediately on signals and suppresses duplicate lines. |
| ✅ 3.3 | Lyrics view only re-renders when the active line changes (was every 100 ms). |

## 4. Visual polish

| # | Change |
|---|--------|
| ✅ 4.1 | Visualizer: 24-level smooth bars using `▁▂▃▄▅▆▇█`, more bars, themed border, gently decays to rest when paused. |
| ✅ 4.2 | Header shows a status-coloured dot, player name and shuffle/loop indicators. |
| ✅ 4.3 | Live streams show `LIVE` with an empty bar instead of a frozen half-filled bar. |
| ✅ 4.4 | Player picker shows playback state icon and current track for each player; duplicate names (e.g. two Firefox instances) are disambiguated. |
| ✅ 4.5 | Lyrics: header no longer shows a dangling ` - `, instrumental tracks show "♪ Instrumental ♪" rather than an error, empty LRC lines render as `♪`, upcoming line dimmed less than past lines. |
| ✅ 4.6 | Full help overlay with key/description columns (instead of a single run-on line). |
| ✅ 4.7 | Volume bar uses fine-grained blocks; 0 % shows a muted icon. |

## 5. Quality-of-life

| Key / flag | Feature |
|------------|---------|
| ✅ `?` | Help (alias of `h`). |
| ✅ `0`–`9` | Jump to 0 %–90 % of the track (`SetPosition`). |
| ✅ `{` / `}` | Seek ±30 s (`[`/`]` stay ±5 s). |
| ✅ `m` | Mute / restore previous volume. |
| ✅ `z` / `r` | Toggle shuffle / cycle loop mode (None → Playlist → Track). |
| ✅ `<` / `>` | Shift synced-lyric timing by ∓250 ms (for badly-timed LRCs); offset shown in header. |
| ✅ `f` | Lyrics: resume auto-follow after manual scroll. `g`/`G` jump to top/bottom. |
| ✅ mouse | Click on the progress bar to seek. |
| ✅ `--format` | Plain-text stream template for Polybar/i3blocks/tmux. |
| ✅ `--interval` | Stream refresh interval. |

## 6. Testing

| # | Change |
|---|--------|
| ✅ 6.1 | Tests for LRC parsing edge cases (metadata tags, `[offset:]`, `mm:ss:xx`, word timestamps, plain fallback), LRCLIB ranking and title clean-up, lyrics cache + sidecar lookup against a fake LRCLIB server, URL→path decoding, rate-aware interpolation, D-Bus number normalisation, theme contrast, stream JSON/template output, layouts fitting 36×16 → 120×40, and the click-to-seek hitbox. |

## Deferred (not in this branch)

- Bubble Tea / Lip Gloss v2 migration (breaking APIs; worth a separate PR).
- Config file (custom keymap, default player, colours).
- Album-art rendering in terminals that support the Kitty/Sixel graphics protocols.
- Release automation (goreleaser + AUR package).

## Verification

- `make check` (go vet, gofmt, all unit tests) passes.
- Exercised live against `mpv` (silent fixture) in a detached tmux session: play/pause, ±5 s / ±30 s seeks, `0`–`9` jumps, click-to-seek, volume, mute/restore, shuffle and loop all confirmed via `playerctl`. Also tested `--list`, `--stream` and `--format` against a browser player, plus help, picker and resize rendering at 90×26 and 44×18.
