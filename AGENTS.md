# AGENTS.md

Coding conventions and project-specific knowledge for AI agents working on Pincer. Adherence to these conventions is a prerequisite for accepting pull requests.

## Build and test

```bash
go build ./...          # Build everything
go test ./...           # Run all tests (no device needed)
go test ./... -v        # Verbose — shows parsed fixture data
go run ./src/pincer     # Run the CLI
```

Tests must pass before committing. There are currently 35 tests across 7 test packages.

## Documentation organization

This project enforces a strict separation between project-level and driver-level documentation. **Pull requests that put driver-specific content in top-level files, or project-level content in driver subdirectories, will not be accepted.**

### What lives at the top level

| File | Purpose |
|------|---------|
| `README.md` | Project introduction, install, architecture, quick command reference, testing overview. Must not contain driver-specific element IDs, screen detection logic, fixture notes, or output examples. |
| `AGENTS.md` | Coding conventions, error patterns, testing patterns, documentation rules. References driver READMEs for app-specific details. |
| `PLAN.md` | Design document and roadmap. |

### What lives in each driver's README

Every driver directory (`src/pincer/drivers/<app>/`) must have a `README.md` containing:

1. **Package name** — the Android package identifier
2. **Commands** — table of available CLI commands with flags
3. **Screens** — table of screen states with detection heuristics and key indicators
4. **Element ID quirks** — resource ID format, notable IDs, any non-obvious parsing logic
5. **Fixture notes** — what each fixture file captures, known gaps
6. **Output examples** — representative JSON output for each command

This is the **only** place for app-specific knowledge. When an agent needs to understand how Grab's restaurant cards are structured or why Shopee uses bare resource IDs, the answer is in the driver's README — not in AGENTS.md or the top-level README.

### Adding a new driver

When adding a new driver, create its README following the structure above before submitting the PR. Use an existing driver README as the template.

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
- `README.md` — driver-specific documentation (see above)
- `driver.go` — screen detection (`DetectScreen` function), navigation methods, the driver struct
- `commands/*.go` — one file per domain (food, auth, chat, cart, search). Each command is a standalone function that takes a context, driver, and flags.

## CLI conventions

Every cobra command must have:
- `Short` — one-line description (shown in parent's help)
- `Long` — multi-line description explaining behavior, output, and edge cases
- `Example` — at least one runnable example, including piping to `jq` where useful

App-level commands (e.g., `grabCmd`) must list available domains in their `Long` text. Leaf commands must document required flags and what the output contains.

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
if err := driver.EnsureAppRunning(ctx); err != nil {
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

### Fixture scrubbing is mandatory

**Never commit raw fixture dumps that contain real personal or account data.** Before a fixture is added or updated, scrub or replace all user-derived content with synthetic placeholders:

- Chat names, message bodies, usernames, profile names
- Email addresses, phone numbers, physical addresses, government IDs, order numbers, license plates
- Account/profile fields, balances, birthdays, saved locations, shopping carts, purchase history, recent searches, and any other behavioral data tied to a real person

If a fixture must preserve structure, keep the UI shape and element IDs but replace the visible values with generated equivalents. If unsanitized fixture data is committed by mistake, fixing the working tree is **not sufficient** — rewrite the affected git history and force-push the cleaned refs.

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

Each driver has a `DetectScreen(finder) Screen` function that identifies which screen the app is showing. Detection uses heuristics — specific resource IDs, text content, or content descriptions that are stable across app versions. The specifics are documented in each driver's README.

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
3. Create `src/pincer/drivers/<app>/README.md` following the required structure (see Documentation organization above)
4. Create `src/pincer/cmd/<app>.go` with cobra commands — must include `Long` and `Example` on every command
5. Capture fixtures from a real device and save to `tests/fixtures/<app>/`
6. Write fixture-driven tests
7. Add a case to `TestAIAssistantAllCommandsCoverage` in `tests/e2e_test.go`

The driver struct should have `Dev core.Device`, `Workflow *core.Workflow`, and `Cache *core.Cache`. Accept `core.Device` in the constructor.

## Adding a new command to an existing driver

1. Add a function in `drivers/<app>/commands/<domain>.go`
2. Wire it to cobra in `cmd/<app>.go` — include `Long` and `Example`
3. Capture relevant fixtures if the existing ones don't cover the new screen
4. Write a test in `drivers/<app>/commands/<domain>_test.go`
5. Add a case to `TestAIAssistantAllCommandsCoverage` in `tests/e2e_test.go`
6. Update the driver's README with the new command, any new screens, and output examples

## Things to watch out for

- `Element.Tap` requires a `context.Context` as its first argument. Don't pass nil.
- `ADB.TypeText` passes arguments directly to `exec.Command` (not through a shell string) to prevent injection. If you add new ADB methods that take user input, do the same.
- `time.Sleep` calls in drivers don't respect context cancellation. This is a known trade-off — the sleeps are short (1-2s) and the added complexity of `select` at every call site wasn't worth it. If you're adding a sleep longer than 3 seconds, use a context-aware wait instead.
- The `Cache` is best-effort. Corrupt or missing cache entries should never fail a command.
- Thai text is prevalent in fixtures (this targets Thai locale apps). `len(s)` counts bytes, not characters. Use `utf8.RuneCountInString` if you need character count.

## Live device testing

The live test suite at `tests/live_test.py` is the ultimate measure of whether Pincer works. It runs 19 test cases (11 documented + 8 adversarial) against a connected Android device across Grab, LINE, and Shopee.

```bash
go build -o pincer ./src/pincer/
python3 tests/live_test.py --seed 42 --include-extra --bin ./pincer
```

The `--seed` flag controls the random execution order. Run with multiple seeds (e.g., 42, 99, 7) to verify order-independence. Use `--passes 2` to repeat the documented cases.

### How to think about live device work

**The device is the source of truth, not the code.** When a command fails, dump the UI (`adb shell "uiautomator dump /sdcard/window_dump.xml" && adb shell "cat /sdcard/window_dump.xml"`) and look at what the device is actually showing. The dump tells you which resource IDs exist, what text is on screen, and what the element hierarchy looks like. Never theorize about what might be wrong without checking.

**Fix the right layer.** When navigation fails, the instinct is to change the navigation method. But navigation methods are shared — making them stricter for one command's needs can break another. If a search command can't find the search bar, fix the recovery in the search command, not in `NavigateToFoodHome`. Put the fix where the problem is specific, not where it's general.

**Reproduce the exact failure sequence.** Failures depend on execution order and app state. If case 6 fails after case 9, run them in that order manually. The same command that works in isolation may fail after a different command leaves the app in an unexpected state.

**Distinguish code bugs from test assumptions.** "LINE unread list returned no chats" isn't a code bug — it's a test that assumes unread chats persist, but a previous test case opened LINE and marked them as read. Before adding complexity to the driver, check if the test's expectation is unrealistic given what ran before it.

**Iterate with the test as the metric.** Make one targeted change, run the suite, read the failures. Don't batch up speculative fixes. Each run tells you what moved and what didn't. If the pass rate didn't improve, your theory was wrong — go look at the device again.

**Apps restore state across launches.** When Shopee resumes, it may restore the scroll position deep in a "You May Also Like" section, hiding the actual cart items above. When Grab resumes from search results, the search bar is gone. These aren't bugs in the app — they're the reality of UI automation. The code must handle the state the app is actually in, not the state you wish it was in.

**One retry in the right place beats five retries in the wrong place.** The Grab search bar fix is a single if-else: if `findSearchBar` fails, press back, re-navigate, try once more. That's it. Don't add retry loops to navigation methods "just in case" — that creates loops (FOOD_RESULTS → back → HOME → tap Food → FOOD_RESULTS → back → ...).

### Common real-device failure patterns

| Symptom | Cause | Fix pattern |
|---------|-------|-------------|
| 0 results despite correct screen | App restored scroll position below the data | Scroll to top before parsing |
| `element_not_found` on second run | Previous results screen has different UI | Recovery path: press back, re-navigate, retry |
| Test fails depending on order | Earlier test changed app state (read chats, navigated away) | Make tests resilient to prior state; fall back to less-specific queries |
| Cart/search returns empty after app switch | Content hasn't loaded yet after activity resume | `WaitForElement` for data-bearing elements, not just header text |
| `uiautomator dump` returns SystemUI | App startup animation still playing | `appStartupSettleDelay` (2s) after launch; retry with `containsNonSystemPackage` check |
| Text entry corrupted | Gboard autocomplete mutating injected text | Use ADB Keyboard IME; don't restore Gboard after injection |
