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

**How it works:** The game writes the authenticated convene URL to `Client.log` (XOR-encrypted in current versions) and/or `debug.log`. This tool searches both files, decrypts `Client.log` using the known XOR cipher, and returns the most recent URL found.

**Supported install locations (auto-detected):**

| OS | Path |
|----|------|
| Linux | `~/.local/share/Steam/steamapps/common/Wuthering Waves/` |
| macOS | `~/Library/Application Support/Steam/steamapps/common/Wuthering Waves/` |
| Windows | `C:\Program Files (x86)\Steam\steamapps\common\Wuthering Waves\` |

Both the base directory and the `Wuthering Waves Game` subdirectory are checked automatically.

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
