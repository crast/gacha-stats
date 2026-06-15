# gacha-stats

CLI tools for extracting gacha/wish/convene history URLs from game files on Linux (and other platforms).

## Programs

### gi-get-stats

Extracts the GI wish history URL from the game's local web cache and copies it to the clipboard, ready to paste into a tracker like [paimon.moe](https://paimon.moe) or [wishing.app](https://wishing.app).


**Supported install locations (auto-detected):**
- Twintail— `~/.local/share/twintaillauncher/games/hk4e_global/<uuid>/`
- Steam — `~/.local/share/Steam/steamapps/common/Genshin Impact/`
- `~/Games/Genshin Impact/`

**Usage:**
```
gi-get-stats [-uid <UID>] [-path <game-dir>]
```

| Flag | Description |
|------|-------------|
| `-uid` | Only accept URLs belonging to this player UID (useful if multiple accounts share a machine) |
| `-path` | Path to the Genshin Impact game directory (overrides auto-detection) |

Previously-checked URLs are cached in `$XDG_CACHE_HOME/gi-stats.json` to avoid redundant API calls.

---

### ww-get-stats

Extracts the Wuthering Waves Convene Record URL from the game's log files and copies it to the clipboard, ready to paste into [wuwatracker.com](https://wuwatracker.com).

**Supported install locations (auto-detected):**

* Twintail Launcher
* Default steam install location

(if you installed to a non-standard place, use `-path` for the path to your game install)

**Usage:**
```
ww-get-stats [-path <game-dir>] [-path <another-dir>] ...
```

| Flag | Description |
|------|-------------|
| `-path` | Additional game install path to search (can be specified multiple times) |

> **Note:** You must open **Convene History** in-game at least once before running this tool so the game writes the URL to its logs.

---

## Building

Requires Go 1.24+.

```sh
# Build both binaries into the current directory
go build ./cmd/gi-get-stats
go build ./cmd/ww-get-stats
```

Or install them to `$GOPATH/bin`:

```sh
go install ./cmd/gi-get-stats
go install ./cmd/ww-get-stats
```

## License

GPL-3.0 — see [LICENSE.txt](LICENSE.txt).
