# Pincer

Pincer automates Android apps via their accessibility APIs and exposes the results as structured CLI commands. Each app gets a "driver" — a module that translates high-level intent into UIAutomator sequences and parses the screen into JSON.

The goal: let an AI agent (or human) interact with apps like Grab, LINE, and Shopee without touching raw UI state.

```bash
pincer grab food search --query "pad thai"
pincer line chat list --unread --limit 5
pincer shopee cart list
```

Output is always JSON to stdout. Errors are JSON to stderr with a non-zero exit code.

```json
{"ok": true, "data": {"restaurants": [{"name": "...", "promo": "20% off"}]}}
```

```json
{"ok": false, "error": "not_logged_in", "message": "App requires login"}
```

## Requirements

- Go 1.21+
- `adb` in your PATH, connected to an Android device or emulator
- Target apps installed on the device

## Install

```bash
git clone https://github.com/scomma/pincer.git
cd pincer
go build -o pincer ./src/pincer
```

## Discovering commands

Pincer's CLI is self-documenting at every level. Run any command with `--help` to see what's available:

```bash
pincer --help                    # List all supported apps
pincer grab --help               # List Grab domains (food, auth)
pincer grab food --help          # List food commands
pincer grab food search --help   # Full docs for a specific command
```

The general pattern is:

```
pincer <app> <domain> <action> [flags]
```

## Supported drivers

Each driver lives in `src/pincer/drivers/<app>/` and has its own README with command reference, screen detection details, element ID quirks, fixture notes, and output examples.

| Driver | App | README |
|--------|-----|--------|
| **grab** | Grab — food delivery, transport | [drivers/grab/README.md](src/pincer/drivers/grab/README.md) |
| **line** | LINE — messaging | [drivers/line/README.md](src/pincer/drivers/line/README.md) |
| **shopee** | Shopee — e-commerce | [drivers/shopee/README.md](src/pincer/drivers/shopee/README.md) |

### Quick reference

| Command | Description |
|---------|-------------|
| `pincer grab food search [--query TEXT]` | List nearby restaurants |
| `pincer grab auth status` | Check Grab login status |
| `pincer line chat list [--unread] [--limit N]` | List LINE chats |
| `pincer line chat read --chat NAME [--limit N]` | Read messages from a chat |
| `pincer shopee cart list` | List shopping cart items |
| `pincer shopee search --query TEXT` | Search for products |

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--device`, `-d` | auto-detect | ADB device serial |
| `--timeout`, `-t` | 30 | Command timeout in seconds |

## Architecture

```
CLI (cobra)
  pincer grab food search
  pincer line chat list
  pincer shopee cart list
    │
    ▼
Driver layer (one per app)
  GrabDriver    LineDriver    ShopeeDriver
    │               │              │
    ▼               ▼              ▼
Core libraries
  Device      ElementFinder    Workflow     Cache
  (ADB)       (XML parsing)    (wait/retry) (file-based)
    │
    ▼
  Android device via adb
```

**Device** (`core/adb.go`) — The `Device` interface abstracts ADB communication: UI dumps, taps, text input, swipes, screenshots, app launching. The `ADB` struct is the real implementation; `MockDevice` is the test double.

**ElementFinder** (`core/elements.go`) — Parses UIAutomator XML dumps into a tree of `Element` structs, then queries them with composable predicates (`ByID`, `ByText`, `ByContentDesc`, `ByClass`, or arbitrary `Predicate` functions via `All`/`First`).

**Workflow** (`core/workflow.go`) — Reusable automation primitives: `FreshDump`, `WaitForElement`, `WaitForPackage`, `ScrollUntil`, `Retry`, `EnsureApp`.

**Drivers** (`drivers/<app>/`) — Each driver has a `driver.go` (screen detection, navigation) and a `commands/` directory (one file per domain). Drivers accept a `core.Device`, making them testable with `MockDevice` and fixture XML.

## Testing

Tests are fixture-driven — real UIAutomator XML dumps captured from devices, replayed through `MockDevice` without needing a connected phone.

```bash
go test ./...           # Run all tests
go test ./... -v        # Verbose — shows parsed fixture data
```

The test suite has three layers:

- **Unit tests** (`core/elements_test.go`, `drivers/*/`) — XML parsing, screen detection, element queries
- **Command tests** (`drivers/*/commands/`) — restaurant card parsing, chat list parsing, cart item parsing
- **E2e tests** (`tests/e2e_test.go`) — simulates an AI assistant running multiple commands across all three apps, verifying JSON output structure

Tests do not require a device. Integration tests against real apps are manual and not run in CI.

## Project structure

```
pincer/
├── src/pincer/
│   ├── main.go
│   ├── cmd/                  # CLI commands (cobra)
│   ├── core/                 # Shared libraries
│   │   ├── adb.go            # Device interface + ADB impl
│   │   ├── device_mock.go    # Test double
│   │   ├── elements.go       # XML parsing + element queries
│   │   ├── workflow.go       # Wait/retry/scroll primitives
│   │   ├── cache.go          # File-based state cache
│   │   └── driver.go         # Driver interface + error types
│   └── drivers/
│       ├── grab/             # Grab driver + README
│       ├── line/             # LINE driver + README
│       └── shopee/           # Shopee driver + README
├── tests/
│   ├── e2e_test.go
│   └── fixtures/
├── PLAN.md                   # Full design document
├── AGENTS.md                 # Coding conventions (required reading for contributors)
├── go.mod
└── go.sum
```

## Status

Phase 1 (Grab read path) and Phase 2 (LINE + Shopee read paths) are complete. Write actions (ordering food, sending messages, checkout) are not yet implemented. See `PLAN.md` for the full roadmap.

## License

Private project. Not yet licensed for distribution.
