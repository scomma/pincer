package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Device is the interface for interacting with an Android device.
type Device interface {
	DumpUI(ctx context.Context) (string, error)
	Tap(ctx context.Context, x, y int) error
	TypeText(ctx context.Context, text string) error
	ClearField(ctx context.Context) error
	KeyEvent(ctx context.Context, key string) error
	Swipe(ctx context.Context, x1, y1, x2, y2, durationMS int) error
	Screenshot(ctx context.Context) ([]byte, error)
	LaunchApp(ctx context.Context, pkg string) error
	CurrentPackage(ctx context.Context) (string, error)
	IsScreenOn(ctx context.Context) (bool, error)
	WakeScreen(ctx context.Context) error
}

// ADB handles communication with an Android device via adb.
type ADB struct {
	DeviceID string
}

const adbKeyboardID = "com.android.adbkeyboard/.AdbIME"

// NewADB creates a new ADB instance. If deviceID is empty, uses the default device.
func NewADB(deviceID string) *ADB {
	return &ADB{DeviceID: deviceID}
}

func (a *ADB) args(cmd ...string) []string {
	if a.DeviceID != "" {
		return append([]string{"-s", a.DeviceID}, cmd...)
	}
	return cmd
}

func (a *ADB) run(ctx context.Context, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, "adb", a.args(args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("adb %s: %w: %s", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

// Shell runs a shell command on the device and returns its output.
func (a *ADB) Shell(ctx context.Context, cmd string) (string, error) {
	out, err := a.run(ctx, "shell", cmd)
	if err != nil {
		return "", fmt.Errorf("adb shell %q: %w", cmd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// DumpUI captures the current UI hierarchy XML from the device.
// Retries up to 3 times on transient errors (signal killed, UI not idle).
func (a *ADB) DumpUI(ctx context.Context) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		_, err := a.Shell(ctx, "uiautomator dump /sdcard/window_dump.xml")
		if err != nil {
			lastErr = fmt.Errorf("uiautomator dump: %w", err)
			time.Sleep(1 * time.Second)
			continue
		}

		args := a.args("shell", "cat", "/sdcard/window_dump.xml")
		c := exec.CommandContext(ctx, "adb", args...)
		out, err := c.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("reading dump: %w: %s", err, string(out))
			time.Sleep(1 * time.Second)
			continue
		}

		xml := string(out)

		// If every package in the dump is SystemUI, the app's UI tree wasn't
		// captured — usually because the app wasn't idle. Wait and retry.
		// We check for non-system packages rather than maintaining an
		// allowlist of supported app packages.
		if strings.Contains(xml, "com.android.systemui") &&
			!containsNonSystemPackage(xml) {
			lastErr = fmt.Errorf("dump captured SystemUI instead of app (attempt %d)", attempt+1)
			time.Sleep(2 * time.Second)
			continue
		}

		return xml, nil
	}

	return "", fmt.Errorf("DumpUI failed after %d attempts: %w", maxRetries, lastErr)
}

// Tap taps a point on the screen.
func (a *ADB) Tap(ctx context.Context, x, y int) error {
	_, err := a.Shell(ctx, fmt.Sprintf("input tap %d %d", x, y))
	return err
}

// Android keycode constants used in ClearField.
const (
	keycodeCtrlLeft = "113" // KEYCODE_CTRL_LEFT
	keycodeA        = "29"  // KEYCODE_A
	keycodeDel      = "67"  // KEYCODE_DEL
)

// ClearField clears text from the currently focused input field.
// Tries Ctrl+A then Delete first (fast path), falls back to
// cursor-to-end plus repeated backspaces if the device doesn't
// support keycombination.
func (a *ADB) ClearField(ctx context.Context) error {
	// Fast path: Ctrl+A (select all) then Delete.
	if _, err := a.run(ctx, "shell", "input", "keycombination", keycodeCtrlLeft, keycodeA); err == nil {
		time.Sleep(150 * time.Millisecond)
		if err := a.KeyEvent(ctx, "KEYCODE_DEL"); err == nil {
			time.Sleep(150 * time.Millisecond)
			return nil
		}
	}
	// Slow path: move to end of field, then send 40 backspaces.
	if err := a.KeyEvent(ctx, "KEYCODE_MOVE_END"); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	// 40 KEYCODE_DEL events in a single shell call to avoid per-call overhead.
	dels := strings.Repeat(keycodeDel+" ", 40)
	if _, err := a.Shell(ctx, "input keyevent "+strings.TrimSpace(dels)); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

// TypeText types text on the device. Prefers the ADB Keyboard IME
// which bypasses soft-keyboard autocorrect. Falls back to `input text`,
// then per-character entry. Switching the IME also dismisses the soft
// keyboard, avoiding the need for callers to send KEYCODE_BACK.
func (a *ADB) TypeText(ctx context.Context, text string) error {
	if err := a.typeWithADBKeyboard(ctx, text); err == nil {
		return nil
	}
	// ADB Keyboard unavailable — hide the soft keyboard by switching
	// to a no-op IME state so it doesn't corrupt input text injection.
	// This is safer than KEYCODE_BACK which can close the entire overlay.
	a.hideSoftKeyboard(ctx)

	if _, err := a.run(ctx, "shell", "input", "text", escapeADBInputText(text)); err == nil {
		return nil
	}

	for _, r := range text {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if unicode.IsSpace(r) {
			if err := a.KeyEvent(ctx, "KEYCODE_SPACE"); err != nil {
				return err
			}
			time.Sleep(120 * time.Millisecond)
			continue
		}
		if err := a.typeRune(ctx, r); err != nil {
			return err
		}
		time.Sleep(90 * time.Millisecond)
	}
	return nil
}

// hideSoftKeyboard dismisses the on-screen keyboard without sending
// KEYCODE_BACK (which can close the containing overlay/dialog).
func (a *ADB) hideSoftKeyboard(ctx context.Context) {
	// "input keyevent 111" is KEYCODE_ESCAPE which hides the keyboard
	// on most devices without closing the activity.
	_ = a.KeyEvent(ctx, "KEYCODE_ESCAPE")
	time.Sleep(200 * time.Millisecond)
}

func (a *ADB) typeWithADBKeyboard(ctx context.Context, text string) error {
	if _, err := a.run(ctx, "shell", "ime", "enable", adbKeyboardID); err != nil {
		return err
	}
	if _, err := a.run(ctx, "shell", "ime", "set", adbKeyboardID); err != nil {
		return err
	}
	// Do NOT restore the original IME here. Switching back to Gboard
	// immediately causes it to autocomplete/mutate the injected text.
	// The ADB keyboard has no visible UI and no autocomplete, so
	// leaving it active is safe for the duration of the automation.

	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	if _, err := a.run(ctx, "shell", "am", "broadcast", "-a", "ADB_INPUT_B64", "--es", "msg", encoded); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)
	return nil
}



func (a *ADB) typeRune(ctx context.Context, r rune) error {
	if key := runeKeyCode(r); key != "" {
		return a.KeyEvent(ctx, key)
	}
	escaped := escapeADBInputText(string(r))
	if _, err := a.run(ctx, "shell", "input", "text", escaped); err != nil {
		return err
	}
	return nil
}

func runeKeyCode(r rune) string {
	switch {
	case r >= 'a' && r <= 'z':
		return "KEYCODE_" + strings.ToUpper(string(r))
	case r >= 'A' && r <= 'Z':
		return "KEYCODE_" + string(r)
	case r >= '0' && r <= '9':
		return "KEYCODE_" + string(r)
	}

	switch r {
	case '-':
		return "KEYCODE_MINUS"
	case '.':
		return "KEYCODE_PERIOD"
	case ',':
		return "KEYCODE_COMMA"
	case '/':
		return "KEYCODE_SLASH"
	case '@':
		return "KEYCODE_AT"
	case '\'':
		return "KEYCODE_APOSTROPHE"
	default:
		return ""
	}
}

func escapeADBInputText(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch r {
		case ' ':
			b.WriteString("%s")
		case '\\', '"', '\'', '(', ')', '<', '>', '|', ';', '&', '*', '$', '!', '?', '#', '%':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if unicode.IsPrint(r) {
				b.WriteRune(r)
				continue
			}
			b.WriteString("\\u")
			b.WriteString(strconv.FormatInt(int64(r), 16))
		}
	}
	return b.String()
}

// KeyEvent sends a key event to the device.
func (a *ADB) KeyEvent(ctx context.Context, key string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("input keyevent %s", key))
	return err
}

// Swipe performs a swipe gesture.
func (a *ADB) Swipe(ctx context.Context, x1, y1, x2, y2, durationMS int) error {
	_, err := a.Shell(ctx, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, durationMS))
	return err
}

// Screenshot captures a PNG screenshot and returns the raw bytes.
func (a *ADB) Screenshot(ctx context.Context) ([]byte, error) {
	args := a.args("exec-out", "screencap", "-p")
	c := exec.CommandContext(ctx, "adb", args...)
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	return out, nil
}

// LaunchApp launches an app by package name.
func (a *ADB) LaunchApp(ctx context.Context, pkg string) error {
	_, err := a.Shell(ctx, fmt.Sprintf("monkey -p %s -c android.intent.category.LAUNCHER 1", pkg))
	return err
}

// CurrentPackage returns the package name of the currently focused app.
func (a *ADB) CurrentPackage(ctx context.Context) (string, error) {
	out, err := a.Shell(ctx, "dumpsys window | grep -E 'mCurrentFocus|mFocusedApp'")
	if err != nil {
		return "", err
	}
	// Parse package from output like "mCurrentFocus=Window{... com.package/activity}"
	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, "/"); idx > 0 {
			// Find the package before the slash
			parts := strings.Fields(line[:idx])
			if len(parts) > 0 {
				return parts[len(parts)-1], nil
			}
		}
	}
	return "", fmt.Errorf("could not determine current package from: %s", out)
}

// containsNonSystemPackage returns true if the XML dump contains any
// package attribute that isn't a system/framework package.
func containsNonSystemPackage(xml string) bool {
	// Look for package="..." attributes that aren't android system packages.
	for _, seg := range strings.Split(xml, "package=\"") {
		idx := strings.Index(seg, "\"")
		if idx <= 0 {
			continue
		}
		pkg := seg[:idx]
		if pkg != "" &&
			!strings.HasPrefix(pkg, "com.android.") &&
			!strings.HasPrefix(pkg, "android") &&
			!strings.HasPrefix(pkg, "com.google.android.inputmethod") &&
			!strings.HasPrefix(pkg, "com.google.android.apps.nexuslauncher") {
			return true
		}
	}
	return false
}

// IsScreenOn checks whether the device display is currently on.
func (a *ADB) IsScreenOn(ctx context.Context) (bool, error) {
	out, err := a.Shell(ctx, "dumpsys power | grep mWakefulness")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "Awake"), nil
}

// WakeScreen turns the display on if it is off.
func (a *ADB) WakeScreen(ctx context.Context) error {
	on, err := a.IsScreenOn(ctx)
	if err != nil {
		return err
	}
	if on {
		return nil
	}
	// WAKEUP turns the screen on without toggling like POWER would.
	if err := a.KeyEvent(ctx, "KEYCODE_WAKEUP"); err != nil {
		return err
	}
	// Brief pause to let the display power on.
	time.Sleep(500 * time.Millisecond)
	// Dismiss the lock screen by swiping up (works on swipe-to-unlock).
	return a.Swipe(ctx, 540, 1600, 540, 800, 300)
}

// WaitForDevice blocks until a device is connected or the context expires.
func (a *ADB) WaitForDevice(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := a.args("wait-for-device")
	c := exec.CommandContext(ctx, "adb", args...)
	return c.Run()
}
