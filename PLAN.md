# Pincer 🦀 — App Driver Framework

> Named after a crab's grip: precise, persistent, gets the job done.

## Overview

Build a framework for automating Android apps via their accessibility APIs, exposing high-level domain commands as a CLI. Each app gets its own "driver" — a module that translates intent-level commands into UI automation sequences.

**Goal:** Let an LLM agent (or human) interact with apps like Grab, LINE, Shopee via simple commands like:

```bash
pincer grab food search --query "lunch" --max-time 30
pincer grab food menu --restaurant-id abc123
pincer line chat list --unread
pincer line chat read --chat "Family Direct" --limit 20
```

Output is always structured (JSON), never raw UI state.

Initial scope is intentionally narrow:
- Accessibility-first apps only
- Read-oriented commands first
- OCR-heavy hostile apps are explicitly out of v1

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                CLI (pincer <app> <command>)             │
├─────────────────────────────────────────────────────────┤
│                    Driver Framework                      │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐        │
│  │ GrabDriver  │ │ LineDriver  │ │ ShopeeDriver│  ...   │
│  └─────────────┘ └─────────────┘ └─────────────┘        │
├─────────────────────────────────────────────────────────┤
│                    Core Libraries                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────┐  │
│  │ UIAuto   │ │ Selector │ │ Workflow │ │ ADB        │  │
│  │ mator    │ │ / Wait   │ │ Helpers  │ │ Transport  │  │
│  └──────────┘ └──────────┘ └──────────┘ └────────────┘  │
├─────────────────────────────────────────────────────────┤
│                 Android Device (ADB)                     │
└─────────────────────────────────────────────────────────┘
```

**Transport options (abstracted):**
- USB ADB to physical phone
- ADB to emulator (BlueStacks, Android Studio)
- Future: on-device daemon for lower latency

---

## Core Framework Components

### 1. ADB Transport (`core/adb.go`)

Handles communication with device:

```go
type ADB struct {
    DeviceID string
}

func (a *ADB) Shell(cmd string) (string, error)
func (a *ADB) DumpUI() (*UIDump, error)
func (a *ADB) Tap(x int, y int) error
func (a *ADB) TypeText(text string) error
func (a *ADB) KeyEvent(key string) error
func (a *ADB) Swipe(x1, y1, x2, y2, durationMS int) error
func (a *ADB) Screenshot() ([]byte, error)
func (a *ADB) LaunchApp(pkg string) error
func (a *ADB) CurrentPackage() (string, error)
```

### 2. Element Finder (`core/elements.go`)

Query UI elements from dumped XML:

```go
type ElementFinder struct {
    Dump *UIDump
}

func (f *ElementFinder) ByID(resourceID string) *Element
func (f *ElementFinder) ByText(text string, exact bool) *Element
func (f *ElementFinder) ByContentDesc(desc string) *Element
func (f *ElementFinder) ByClass(className string) []*Element
func (f *ElementFinder) All(predicates ...Predicate) []*Element

type Element struct {
    Text        string
    ResourceID  string
    ContentDesc string
    Bounds      Rect
    Clickable   bool
}

func (e *Element) Center() Point
func (e *Element) Tap(adb *ADB) error
```

### 3. Workflow Primitives (`core/workflow.go`)

Start with command-scoped workflows and reusable primitives instead of a formal state machine:

```go
type WorkflowHelpers struct {
    adb *ADB
}

func (w *WorkflowHelpers) WaitForElement(selector Selector, timeout time.Duration) (*Element, error)
func (w *WorkflowHelpers) WaitForPackage(pkg string, timeout time.Duration) error
func (w *WorkflowHelpers) ScrollUntil(match func(*UIDump) bool, limit int) error
func (w *WorkflowHelpers) Retry(op func() error, attempts int, delay time.Duration) error
```

Driver implementations should evolve gradually. If stable navigation and detection patterns emerge across multiple drivers, extract them into shared screen or state abstractions later.

### 4. Base Driver (`core/driver.go`)

Abstract base class for app drivers:

```go
type Driver interface {
    PackageName() string
    EnsureAppRunning(ctx context.Context) error
    EnsureLoggedIn(ctx context.Context) (bool, error)
}
```

---

## CLI Design

Unified entry point with app subcommands:

```bash
# Pattern: pincer <app> <domain> <action> [--options]

# Grab
pincer grab food search [--query TEXT] [--max-time MINS] [--cuisine TYPE]
pincer grab food restaurants [--sort rating|time|distance]
pincer grab food menu --restaurant-id ID
pincer grab food order --restaurant-id ID --items JSON  # Later phase
pincer grab food track [--order-id ID]  # Later phase
pincer grab ride estimate --from ADDR --to ADDR  # Later phase
pincer grab ride book --from ADDR --to ADDR [--type car|bike]  # Later phase
pincer grab wallet balance  # Later phase

# LINE  
pincer line chat list [--unread] [--limit N]
pincer line chat read --chat NAME [--limit N]
pincer line chat send --chat NAME --message TEXT  # Later phase
pincer line chat search --query TEXT

# Shopee
pincer shopee cart list  # Later phase
pincer shopee cart add --product-id ID [--quantity N]  # Later phase
pincer shopee orders list [--status pending|shipped|delivered]  # Later phase
pincer shopee search --query TEXT [--sort price|sales]  # Later phase
```

**Output:**

Always JSON to stdout. Errors as JSON to stderr with non-zero exit.

```json
{
  "ok": true,
  "data": { ... }
}
```

```json
{
  "ok": false,
  "error": "not_logged_in",
  "message": "App requires login. Run: pincer grab auth login"
}
```

---

## Grab Driver Specification

### Package
`com.grabtaxi.passenger`

### Screens

| Screen | Detection | Key Elements |
|--------|-----------|--------------|
| `HOME` | Has Food/Transport/Mart tiles | `tile_flat_view` with "Food" |
| `FOOD_HOME` | Search bar + Delivery/Pickup tabs | "What shall we deliver?" |
| `FOOD_RESULTS` | Restaurant list | `restaurant_list` or cards with ratings |
| `RESTAURANT` | Menu items visible | Restaurant name header + menu items |
| `CART` | Cart items + checkout | `cart_item` elements |
| `LOGIN_PHONE` | Phone input field | "Continue With Mobile Number" |
| `LOGIN_OTP` | OTP input | "Enter the 6-digit code" |
| `LOGIN_PIN` | PIN input | "Enter your PIN" |

### Commands

**`pincer grab food search`**
1. Ensure screen: `FOOD_HOME`
2. If query provided: tap search, type query, submit
3. Parse restaurant cards: name, rating, time, distance, promos
4. Return structured list

**`pincer grab food menu --restaurant-id ID`**
1. Navigate to restaurant (may need to search first, or use deep link if available)
2. Parse menu sections and items: name, price, description, modifiers
3. Return structured menu

**`pincer grab auth status`**
1. Check if logged in (navigate to Account, see if profile loads)
2. Return `{logged_in: bool, phone: str | null}`

**`pincer grab auth login --phone NUMBER`**
1. Navigate to login
2. Enter phone, request OTP
3. Return `{awaiting_otp: true}` — separate command to submit OTP

---

## LINE Driver Specification

### Package
`jp.naver.line.android`

### Key Insight
LINE has good accessibility support. UIAutomator works well.

### Screens

| Screen | Detection |
|--------|-----------|
| `CHATS` | Chat list visible, "Chats" tab selected |
| `CHAT_DETAIL` | Message list + input field |
| `FRIENDS` | Friends tab selected |

### Commands

**`pincer line chat list`**
1. Ensure screen: `CHATS`
2. Parse chat list: name, last message preview, time, unread count, muted
3. Return structured list

**`pincer line chat read --chat NAME`**
1. Find chat by name in list, tap to open
2. Parse visible messages: sender, text, time
3. Return structured messages (most recent N)

**Later-phase write actions**
- `pincer line chat send --chat NAME --message TEXT`
- `pincer grab food order --restaurant-id ID --items JSON`
- Any Shopee mutation flows

---

## Error Handling

### Error Categories

```go
type DriverError struct {
    Code    string
    Message string
}
```

### Retry Logic

- Element not found: retry dump up to 3 times with 500ms delay
- Navigation failed: try alternative path once
- App crash: relaunch once
- Persisted selector cache miss or stale selector: automatically invalidate, fall back to fresh detection, and continue when possible

### Timeout

Each command has default timeout (30s). Configurable via `--timeout`.

---

## State Persistence

Minimal state between commands:
- Cache screen detection heuristics
- Cache element selectors that worked
- Don't cache volatile data (cart contents, etc.)

Store in `~/.pincer/state/<package>/`

Persistence must be best-effort. Corrupt or stale cache entries should be ignored, repaired, or rebuilt automatically without failing the command unless no fallback path succeeds.

---

## Configuration

`~/.pincer/config.yaml`:

```yaml
device: SERIAL_OR_AUTO
timeout_default: 30

apps:
  grab:
    package: com.grabtaxi.passenger
    # Optional overrides
  line:
    package: jp.naver.line.android
```

---

## Testing

### Unit Tests
- Element parsing from sample XML dumps
- Workflow helper behavior
- Driver command parsing against fixtures

### Integration Tests
- Requires emulator or device
- Test each command against real app
- Use snapshot/restore for reproducibility
- Do not automatically run write-action tests in CI or default local test flows

### Fixtures
- Save UIAutomator XML dumps as fixtures
- Replay tests without device
- Note for later: add golden-output validation for machine-facing JSON responses

---

## Deliverables

### Phase 1: Framework + Grab Read Path
1. Core libraries (adb, elements, workflow helpers, cache)
2. CLI scaffold
3. GrabDriver with:
   - `pincer grab food search`
   - `pincer grab food restaurants`
   - `pincer grab food menu`
   - `pincer grab auth status`
4. Tests with fixtures

### Phase 2: LINE + Shopee Read Path
1. LineDriver with chat commands
2. `pincer line chat list`
3. `pincer line chat read`
4. ShopeeDriver read-only exploration
5. `pincer shopee cart list`
6. `pincer shopee orders list [--status pending|shipped|delivered]`
7. `pincer shopee search --query TEXT [--sort price|sales]`
8. `pincer shopee cart add --product-id ID [--quantity N]`
9. Tests

### Phase 3: Write Actions + Expanded Coverage
1. `pincer line chat send`
2. `pincer grab food order` with explicit confirmation gates
3. Shopee checkout flows
4. Manual or opt-in verification flows for write actions

### Phase 4: Polish
1. Better error messages
2. `--verbose` / `--debug` flags
3. Shell completion
4. README + docs
5. Binary release packaging

---

## Technical Notes

### Stack Suggestions

- **Language:** Go
- **CLI:** `cobra`
- **ADB:** shell out to `adb` via `os/exec`
- **XML parsing:** `encoding/xml`
- **Testing:** Go `testing` package with fixture-driven tests
- **OCR:** later phase only, behind an interface if added

### Don't Over-Engineer

- No async unless needed
- No database — flat files for state
- No web server — CLI only for v1
- No LLM — deterministic code paths
- Don't introduce a formal state machine until stable cross-driver patterns are proven

### Repository Structure

```
pincer/
├── README.md
├── PLAN.md              # This file
├── go.mod
├── src/
│   └── pincer/
│       ├── main.go          # Entry point
│       ├── core/
│       │   ├── adb.go
│       │   ├── elements.go
│       │   ├── workflow.go
│       │   └── cache.go
│       └── drivers/
│           ├── base.go
│           ├── grab/
│           │   ├── driver.go
│           │   └── commands/
│           │       ├── food.go
│           │       └── auth.go
│           ├── line/
│           │   └── ...
├── tests/
│   ├── fixtures/        # XML dumps, screenshots
│   └── ...
└── examples/
    └── order_lunch.sh   # Demo script
```

---

## Success Criteria

An LLM agent should be able to:

```bash
# Check what's for lunch
pincer grab food search --max-time 30 | jq '.data.restaurants[:3]'

# Inspect a menu
pincer grab food menu --restaurant-id R123

# Read LINE messages
pincer line chat list --unread
pincer line chat read --chat "Work Group" --limit 10
```

No screenshots. No coordinates. No "tap the third button." Just intent → structured response.

---

## Context

This project emerged from exploring Android automation via ADB for apps without official APIs (Grab, LINE). The raw UIAutomator approach works but operates at the wrong abstraction layer for an LLM agent — too much UI state leaks through.

Pincer provides the missing translation layer: **intent in, structured data out**.
