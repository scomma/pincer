# Live Device Test Plan

Manual test cases verified against a Pixel 6a. These complement the
synthetic fixture tests in `e2e_test.go` and `robustness_test.go` and
should be re-run after any change to navigation, screen detection,
text input, or scroll logic.

## Prerequisites

- Device connected via USB (`adb devices` shows it)
- ADB Keyboard installed (`adb shell ime list -a | grep adbkeyboard`)
- Grab, LINE, and Shopee installed and logged in
- Animations disabled (Settings → Developer → all animation scales → 0)

## Test Cases

### 1. Screen off → Grab food search (no query)

```bash
adb shell "input keyevent KEYCODE_POWER"   # turn screen off
sleep 2
./pincer grab food search -t 90
```

**Expected:** Wakes screen, launches Grab, navigates to food home,
returns restaurants with names and optional promo text. No ad labels
("Ad", "Only at Grab") in restaurant names.

---

### 2. Grab food search with query

```bash
./pincer grab food search -q "pad thai" -t 90
```

**Expected:** Opens search, types "pad thai" via ADB Keyboard IME,
submits search, returns relevant pad thai restaurants. The `query`
field in the response matches the input.

**Known fragility:** If ADB Keyboard is not installed, text entry
falls back to `input text` which can be corrupted by Gboard
autocomplete.

---

### 3. Grab auth status

```bash
./pincer grab auth status -t 90
```

**Expected:** Returns `logged_in: true` with a screen name. Should
work from any Grab screen (the command navigates internally).

---

### 4. Wrong app → LINE chat list (unread)

```bash
adb shell "monkey -p com.grabtaxi.passenger -c android.intent.category.LAUNCHER 1"
sleep 3
./pincer line chat list --unread --limit 5 -t 90
```

**Expected:** Launches LINE from Grab, navigates to chat list, returns
up to 5 chats with `unread_count > 0`. Each chat has name, last_message,
time, unread_count.

---

### 5. LINE chat read (scroll to find)

```bash
./pincer line chat read --chat "<visible chat name>" -t 90
```

**Expected:** Scrolls to top of chat list, scrolls down to find the
target chat, opens it, returns messages. Use a chat name from the
output of test 4.

**Known limitation:** If the chat name doesn't exist or is too far
down the list (beyond 10 scrolls), returns `chat_not_found`.

---

### 6. Shopee cart list (full scroll)

```bash
./pincer shopee cart list -t 90
```

**Expected:** Navigates to Shopee cart, scrolls through all items,
returns items with shop, name, variation, price, quantity. Item count
should match what's visible on the device (verify manually).

**Known noise:** "Only N left" and "Flash Sale" labels occasionally
leak through as items.

---

### 7. Shopee search with query

```bash
./pincer shopee search -q "usb cable" -t 90
```

**Expected:** Navigates to Shopee home, taps search, types query,
submits, returns products with name, price, discount, sold count.

---

### 8. All apps killed + screen off → cold launch

```bash
adb shell "am force-stop com.grabtaxi.passenger"
adb shell "am force-stop jp.naver.line.android"
adb shell "am force-stop com.shopee.th"
adb shell "input keyevent KEYCODE_POWER"
sleep 2
./pincer line chat list --limit 3 -t 90
```

**Expected:** Wakes screen, cold-launches LINE, navigates to chat
list, returns 3 chats. This is the worst-case startup scenario.

---

### 9. Rapid cross-app sequence

```bash
./pincer grab food search -t 90
./pincer shopee cart list -t 90
./pincer line chat list --unread --limit 2 -t 90
./pincer grab food search -q "burger" -t 90
```

**Expected:** All four commands succeed in sequence. Each navigates
away from the previous app, launches the correct one, and returns
valid results.

---

### 10. Deep sub-screen recovery

```bash
# Navigate Grab into a deep sub-screen manually (e.g., tap a
# restaurant, then a menu item), then run:
./pincer grab food search -t 90
```

**Expected:** Presses back repeatedly until reaching the food home,
then returns restaurants. Should not get stuck or timeout.

---

## Error output contract

All error responses must go to stderr (not stdout) as JSON:

```bash
./pincer shopee search 2>/dev/null   # should produce no stdout
./pincer shopee search 2>&1 1>/dev/null   # should show error JSON
```

**Expected:** `{"ok": false, "error": "...", "message": "..."}` on
stderr, exit code 1, nothing on stdout.
