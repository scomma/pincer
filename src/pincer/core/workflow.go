package core

import (
	"context"
	"fmt"
	"time"
)

// Workflow provides reusable automation primitives.
type Workflow struct {
	Dev Device
}

// NewWorkflow creates a Workflow bound to a Device.
func NewWorkflow(dev Device) *Workflow {
	return &Workflow{Dev: dev}
}

// FreshDump captures a fresh UI dump and returns an ElementFinder.
func (w *Workflow) FreshDump(ctx context.Context) (*ElementFinder, error) {
	xml, err := w.Dev.DumpUI(ctx)
	if err != nil {
		return nil, err
	}
	return NewElementFinderFromXML([]byte(xml))
}

// WaitForElement polls the UI until an element matching the predicates appears.
func (w *Workflow) WaitForElement(ctx context.Context, timeout time.Duration, predicates ...Predicate) (*Element, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		finder, err := w.FreshDump(ctx)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if el := finder.First(predicates...); el != nil {
			return el, nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	if lastErr != nil {
		return nil, fmt.Errorf("element not found within %v (last error: %w)", timeout, lastErr)
	}
	return nil, fmt.Errorf("element not found within %v", timeout)
}

// WaitForPackage waits until the given package is in the foreground.
func (w *Workflow) WaitForPackage(ctx context.Context, pkg string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		current, err := w.Dev.CurrentPackage(ctx)
		if err == nil && current == pkg {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("package %s not in foreground within %v", pkg, timeout)
}

// Swipe coordinates assuming a 1080×2400 screen. Centralized so there's
// one place to change when supporting different screen resolutions.
const (
	SwipeX     = 540
	SwipeDownY1 = 1600
	SwipeDownY2 = 800
	SwipeUpY1   = 400
	SwipeUpY2   = 1600
	SwipeDurMS  = 300
)

// ScrollDown performs a downward scroll gesture.
func (w *Workflow) ScrollDown(ctx context.Context) error {
	return w.Dev.Swipe(ctx, SwipeX, SwipeDownY1, SwipeX, SwipeDownY2, SwipeDurMS)
}

// ScrollUp performs an upward scroll gesture.
func (w *Workflow) ScrollUp(ctx context.Context) error {
	return w.Dev.Swipe(ctx, SwipeX, SwipeUpY1, SwipeX, SwipeUpY2, SwipeDurMS)
}

// ScrollUntil scrolls the screen until the match function returns true or the limit is reached.
func (w *Workflow) ScrollUntil(ctx context.Context, match func(*ElementFinder) bool, limit int) error {
	for i := 0; i < limit; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		finder, err := w.FreshDump(ctx)
		if err != nil {
			return err
		}
		if match(finder) {
			return nil
		}
		if err := w.ScrollDown(ctx); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("condition not met after %d scrolls", limit)
}

// BackOrRelaunch presses back and re-launches the app if back left it.
// Used by navigation methods to recover from unknown screens.
func (w *Workflow) BackOrRelaunch(ctx context.Context, pkg string) error {
	if err := w.Dev.KeyEvent(ctx, "KEYCODE_BACK"); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)

	current, _ := w.Dev.CurrentPackage(ctx)
	if current != pkg {
		return w.EnsureApp(ctx, pkg, 10*time.Second)
	}
	return nil
}

// Retry retries an operation up to the given number of attempts.
func (w *Workflow) Retry(op func() error, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := op(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < attempts-1 {
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", attempts, lastErr)
}

// EnsureApp launches the app if it's not already in the foreground.
// It first wakes the screen if the display is off.
func (w *Workflow) EnsureApp(ctx context.Context, pkg string, timeout time.Duration) error {
	// Wake the screen — commands can't proceed with the display off.
	if err := w.Dev.WakeScreen(ctx); err != nil {
		return fmt.Errorf("waking screen: %w", err)
	}

	current, err := w.Dev.CurrentPackage(ctx)
	if err == nil && current == pkg {
		return nil
	}
	if err := w.Dev.LaunchApp(ctx, pkg); err != nil {
		return fmt.Errorf("launching %s: %w", pkg, err)
	}
	if err := w.WaitForPackage(ctx, pkg, timeout); err != nil {
		return err
	}
	// After the package appears in the foreground, wait for the app to
	// finish its startup animations. Without this pause, the first
	// UI dump often captures the system UI overlay instead of the app.
	time.Sleep(3 * time.Second)
	return nil
}
