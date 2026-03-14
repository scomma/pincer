package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Device is the interface for interacting with an Android device.
type Device interface {
	DumpUI(ctx context.Context) (string, error)
	Tap(ctx context.Context, x, y int) error
	TypeText(ctx context.Context, text string) error
	KeyEvent(ctx context.Context, key string) error
	Swipe(ctx context.Context, x1, y1, x2, y2, durationMS int) error
	Screenshot(ctx context.Context) ([]byte, error)
	LaunchApp(ctx context.Context, pkg string) error
	CurrentPackage(ctx context.Context) (string, error)
}

// ADB handles communication with an Android device via adb.
type ADB struct {
	DeviceID string
}

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

// Shell runs a shell command on the device and returns its output.
func (a *ADB) Shell(ctx context.Context, cmd string) (string, error) {
	args := a.args("shell", cmd)
	c := exec.CommandContext(ctx, "adb", args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("adb shell %q: %w: %s", cmd, err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// DumpUI captures the current UI hierarchy XML from the device.
func (a *ADB) DumpUI(ctx context.Context) (string, error) {
	_, err := a.Shell(ctx, "uiautomator dump /sdcard/window_dump.xml")
	if err != nil {
		return "", fmt.Errorf("uiautomator dump: %w", err)
	}
	args := a.args("shell", "cat", "/sdcard/window_dump.xml")
	c := exec.CommandContext(ctx, "adb", args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("reading dump: %w: %s", err, string(out))
	}
	return string(out), nil
}

// Tap taps a point on the screen.
func (a *ADB) Tap(ctx context.Context, x, y int) error {
	_, err := a.Shell(ctx, fmt.Sprintf("input tap %d %d", x, y))
	return err
}

// TypeText types text on the device.
// Uses exec args directly to avoid shell injection.
func (a *ADB) TypeText(ctx context.Context, text string) error {
	// adb's `input text` uses %s for spaces — that's the only substitution needed.
	// By passing args directly to exec (not through a shell string), we avoid injection.
	escaped := strings.ReplaceAll(text, " ", "%s")
	args := a.args("shell", "input", "text", escaped)
	c := exec.CommandContext(ctx, "adb", args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb input text: %w: %s", err, string(out))
	}
	return nil
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

// WaitForDevice blocks until a device is connected or the context expires.
func (a *ADB) WaitForDevice(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := a.args("wait-for-device")
	c := exec.CommandContext(ctx, "adb", args...)
	return c.Run()
}
