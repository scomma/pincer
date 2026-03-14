# AGENTS.md

Coding conventions and project-specific knowledge for AI agents working on Pincer.

## Build and test

```bash
go build ./...          # Build everything
go test ./...           # Run all tests (no device needed)
go test ./... -v        # Verbose — shows parsed fixture data
go run ./src/pincer     # Run the CLI
```

Tests must pass before committing. There are currently 35 tests across 7 test packages.

## How the code is organized

The module root is `github.com/prathan/pincer`. Source lives under `src/pincer/`. This is intentional — don't move it to the repo root.

```
src/pincer/core/       → shared libraries (Device, ElementFinder, Workflow, Cache, errors)
src/pincer/drivers/    → one subdir per app (grab/, line/, shopee/)
src/pincer/cmd/        → cobra CLI commands
tests/fixtures/        → real UIAutomator XML dumps from devices
tests/e2e_test.go      → end-to-end tests using MockDevice
```

Each driver follows the same structure:
- `driver.go` — screen detection (a `DetectScreen` function), navigation methods, the driver struct
- `commands/*.go` — one file per domain (food, auth, chat, cart, search). Each command is a standalone function that takes a context, driver, and flags.

## Key interfaces

### `core.Device`

All device interaction goes through this interface. `ADB` is the real implementation. `MockDevice` is the test double. Drivers accept `core.Device` — never `*ADB` directly.

When adding new device operations, add them to the `Device` interface in `core/adb.go`, implement on `ADB`, and add a no-op/recording stub to `MockDevice`.

### `core.Driver`

The `Driver` interface (`PackageName`, `EnsureAppRunning`, `EnsureLoggedIn`) is defined but drivers are not used polymorphically yet. Each driver struct (e.g., `GrabDriver`) is referenced by its concrete type in commands.

### `core.ElementFinder`

Query elements with `ByID`, `ByText`, `ByContentDesc`, `ByClass`, or composable predicates via `All`/`First`. Predicates are `func(*Element) bool`. Common ones: `HasText`, `HasID`, `HasContentDesc`, `IsClickable`, `IsScrollable`.

`First` returns the first match or nil. `All` returns all matches. Both do a depth-first traversal of the element tree.

## Error conventions

Sentinel errors are **constructor functions**, not values:

```go
// CORRECT — returns a fresh *DriverError each time
return core.ErrNavigation()

// WRONG — would share a mutable pointer
return core.ErrNavigation
```

This prevents shared-mutable-sentinel bugs. All common errors (`ErrNotLoggedIn`, `ErrElementNotFound`, `ErrTimeout`, `ErrNavigation`, `ErrAppNotRunning`) follow this pattern.

When wrapping real errors from ADB or other calls, use `fmt.Errorf` with `%w`:

```go
if err :=driver.EnsureAppRunning(ctx); err != nil {
    return nil, fmt.Errorf("ensure app running: %w", err)
}
```

Do not replace real errors with generic sentinels — the diagnostic info matters.

## JSON output convention

All CLI commands produce this envelope:

```json
{"ok": true, "data": { ... }}
```

Errors:

```json
{"ok": false, "error": "error_code", "message": "Human-readable message"}
```

The `error` field is a machine-readable code (snake_case). The `message` field is for humans. Commands write success JSON to stdout and error JSON to stderr.

## Testing conventions

### Fixture-driven, no device required

Tests parse real UIAutomator XML dumps from `tests/fixtures/`. To test a new command, capture a dump from a real device (`adb exec-out uiautomator dump /dev/tty`) and save it as a fixture.

### MockDevice

`core.MockDevice` records all calls (taps, text input, key events) and returns fixture XML from `DumpUI`. Create one with:

```go
mock := core.NewMockDevice("path/to/fixture.xml", "com.package.name")
driver, _ := grab.NewGrabDriver(mock)
```

Use `NewMockDeviceWithSequence` to cycle through multiple fixtures (simulating screen transitions).

After running a command, inspect what happened:

```go
mock.Taps()       // recorded tap coordinates
mock.TypedTexts() // recorded text input
mock.Calls()      // all method calls in order
```

### Test file placement

- Unit tests go next to the code: `drivers/grab/driver_test.go`
- Command-level tests: `drivers/grab/commands/food_test.go`
- Cross-driver e2e tests: `tests/e2e_test.go`

Each test package has its own `loadFixture` helper. This is duplicated intentionally — it's 6 lines and doesn't warrant a shared package.

### Fixture paths are relative

Tests use relative paths like `"../../../../tests/fixtures/grab/home.xml"`. These work because Go sets the working directory to the package directory during tests. Don't restructure the directory tree without updating these paths.

## Screen detection

Each driver has a `DetectScreen(finder) Screen` function that identifies which screen the app is showing. Detection uses heuristics — specific resource IDs, text content, or content descriptions that are stable across app versions.

When the Grab fixtures were captured, the home screen and food home screen were visually different states but structurally similar (both have the service tiles). The key differentiator is `search_bar_clickable_area` — present on food screens, absent on the pure home screen. Our current fixtures don't include a "pure home" capture.

## App-specific quirks

### Grab (`com.grabtaxi.passenger`)

- Resource IDs use the full package prefix: `com.grabtaxi.passenger:id/duxton_card`
- The food tile has `content-desc="Food, double tap to select"` (accessibility label), which is more reliable than matching the "Food" text
- Restaurant cards use `duxton_card` as the resource ID (Grab's internal component name)
- Restaurant names and promos are child TextViews inside `duxton_card`, with no distinguishing resource IDs — identified by position (first text = name, promo matched by regex)

### LINE (`jp.naver.line.android`)

- Resource IDs use the full package prefix: `jp.naver.line.android:id/name`
- Chat list items are children of `chat_list_recycler_view`
- Each chat item has well-labeled children: `name`, `last_message`, `date`, `unread_message_count`, `member_count`, `notification_off`
- OpenChat groups use a different unread ID: `square_chat_unread_message_count`
- Unread counts can be `"999+"` — the `+` is stripped before parsing
- Chat read uses **exact name match only**. No fuzzy/substring fallback, to prevent opening the wrong conversation.

### Shopee (`com.shopee.th`)

- Resource IDs have **no package prefix** — just bare names like `labelItemName`, `labelShopName`, `labelVariation`. This is different from Grab and LINE. Don't add `com.shopee.th:id/` when querying.
- The `homepage_main_recycler` ID does use the full prefix (`com.shopee.th:id/homepage_main_recycler`)
- Cart items are associated to shops by vertical position — each item's shop is the nearest `labelShopName` above it in Y coordinates
- The `orders.xml` fixture is actually a profile/edit screen, not an orders list. It was captured from the Me tab.

## Navigation pattern

Navigation methods (e.g., `NavigateToFoodHome`, `NavigateToChats`) use a bounded retry loop:

```go
const maxRetries = 3
for attempt := 0; attempt <= maxRetries; attempt++ {
    // dump UI, detect screen, act
}
return core.ErrNavigation()
```

Do not use recursion for navigation. A previous version did and it caused stack overflows on unexpected popups.

## Adding a new driver

1. Create `src/pincer/drivers/<app>/driver.go` with screen detection and navigation
2. Create `src/pincer/drivers/<app>/commands/<domain>.go` with command functions
3. Create `src/pincer/cmd/<app>.go` with cobra commands
4. Capture fixtures from a real device and save to `tests/fixtures/<app>/`
5. Write fixture-driven tests

The driver struct should have `Dev core.Device`, `Workflow *core.Workflow`, and `Cache *core.Cache`. Accept `core.Device` in the constructor.

## Adding a new command to an existing driver

1. Add a function in `drivers/<app>/commands/<domain>.go`
2. Wire it to cobra in `cmd/<app>.go`
3. Capture relevant fixtures if the existing ones don't cover the new screen
4. Write a test in `drivers/<app>/commands/<domain>_test.go`
5. Add a case to `TestAIAssistantAllCommandsCoverage` in `tests/e2e_test.go`

## Things to watch out for

- `Element.Tap` requires a `context.Context` as its first argument. Don't pass nil.
- `ADB.TypeText` passes arguments directly to `exec.Command` (not through a shell string) to prevent injection. If you add new ADB methods that take user input, do the same.
- `time.Sleep` calls in drivers don't respect context cancellation. This is a known trade-off — the sleeps are short (1-2s) and the added complexity of `select` at every call site wasn't worth it. If you're adding a sleep longer than 3 seconds, use a context-aware wait instead.
- The `Cache` is best-effort. Corrupt or missing cache entries should never fail a command.
- Thai text is prevalent in fixtures (this targets Thai locale apps). `len(s)` counts bytes, not characters. Use `utf8.RuneCountInString` if you need character count.
