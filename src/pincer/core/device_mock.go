package core

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// MockDevice is a test double for Device that returns fixture data.
type MockDevice struct {
	mu      sync.Mutex
	pkg     string
	dumps   []string // fixture paths to cycle through
	dumpIdx int
	taps    []Point
	typed   []string
	keys    []string
	calls   []string

	// Configurable behavior for robustness testing.
	screenOn       bool           // whether screen is "on" (default true)
	dumpDelay      time.Duration  // artificial delay before returning UI dump
	dumpErrors     int            // number of DumpUI calls that return errors before succeeding
	dumpErrorCount int            // internal counter
	tapErrors      int            // number of Tap calls that return errors
	tapErrorCount  int            // internal counter
}

// NewMockDevice creates a MockDevice with a single dump fixture.
func NewMockDevice(dumpFixturePath string, currentPkg string) *MockDevice {
	return &MockDevice{
		pkg:      currentPkg,
		dumps:    []string{dumpFixturePath},
		screenOn: true,
	}
}

// NewMockDeviceWithSequence creates a MockDevice that cycles through fixtures.
func NewMockDeviceWithSequence(fixtures []string, currentPkg string) *MockDevice {
	return &MockDevice{
		pkg:      currentPkg,
		dumps:    fixtures,
		screenOn: true,
	}
}

func (m *MockDevice) DumpUI(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, "DumpUI")

	// Simulate transient dump errors.
	if m.dumpErrorCount < m.dumpErrors {
		m.dumpErrorCount++
		return "", fmt.Errorf("mock transient error: UI dump unavailable (attempt %d)", m.dumpErrorCount)
	}

	// Simulate slow UI dumps.
	if m.dumpDelay > 0 {
		m.mu.Unlock()
		select {
		case <-time.After(m.dumpDelay):
		case <-ctx.Done():
			m.mu.Lock()
			return "", ctx.Err()
		}
		m.mu.Lock()
	}

	if len(m.dumps) == 0 {
		return "", fmt.Errorf("no dump fixtures configured")
	}
	path := m.dumps[m.dumpIdx]
	m.dumpIdx = (m.dumpIdx + 1) % len(m.dumps)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("mock DumpUI: %w", err)
	}
	return string(data), nil
}

func (m *MockDevice) Tap(_ context.Context, x, y int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taps = append(m.taps, Point{x, y})
	m.calls = append(m.calls, fmt.Sprintf("Tap(%d,%d)", x, y))

	if m.tapErrorCount < m.tapErrors {
		m.tapErrorCount++
		return fmt.Errorf("mock tap error: input injection failed (attempt %d)", m.tapErrorCount)
	}
	return nil
}

func (m *MockDevice) TypeText(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.typed = append(m.typed, text)
	m.calls = append(m.calls, fmt.Sprintf("TypeText(%q)", text))
	return nil
}

func (m *MockDevice) KeyEvent(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, key)
	m.calls = append(m.calls, fmt.Sprintf("KeyEvent(%q)", key))
	return nil
}

func (m *MockDevice) Swipe(_ context.Context, x1, y1, x2, y2, durationMS int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("Swipe(%d,%d->%d,%d,%d)", x1, y1, x2, y2, durationMS))
	return nil
}

func (m *MockDevice) Screenshot(_ context.Context) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, "Screenshot")
	return []byte{}, nil
}

func (m *MockDevice) LaunchApp(_ context.Context, pkg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pkg = pkg
	m.calls = append(m.calls, fmt.Sprintf("LaunchApp(%q)", pkg))
	return nil
}

func (m *MockDevice) CurrentPackage(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, "CurrentPackage")
	return m.pkg, nil
}

func (m *MockDevice) IsScreenOn(_ context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, "IsScreenOn")
	return m.screenOn, nil
}

func (m *MockDevice) WakeScreen(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, "WakeScreen")
	m.screenOn = true
	return nil
}

// SetDump changes the fixture(s) returned by DumpUI.
func (m *MockDevice) SetDump(paths ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dumps = paths
	m.dumpIdx = 0
}

// SetScreenOn controls whether IsScreenOn returns true or false.
func (m *MockDevice) SetScreenOn(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.screenOn = on
}

// SetDumpDelay adds an artificial delay to DumpUI calls.
func (m *MockDevice) SetDumpDelay(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dumpDelay = d
}

// SetDumpErrors makes the first n DumpUI calls return transient errors.
func (m *MockDevice) SetDumpErrors(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dumpErrors = n
	m.dumpErrorCount = 0
}

// SetTapErrors makes the first n Tap calls return errors.
func (m *MockDevice) SetTapErrors(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tapErrors = n
	m.tapErrorCount = 0
}

// SetPackage changes the package returned by CurrentPackage.
func (m *MockDevice) SetPackage(pkg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pkg = pkg
}

// Reset clears all recorded calls and resets error counters.
func (m *MockDevice) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taps = nil
	m.typed = nil
	m.keys = nil
	m.calls = nil
	m.dumpIdx = 0
	m.dumpErrorCount = 0
	m.tapErrorCount = 0
}

// Taps returns recorded taps.
func (m *MockDevice) Taps() []Point {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Point{}, m.taps...)
}

// TypedTexts returns recorded text inputs.
func (m *MockDevice) TypedTexts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.typed...)
}

// Calls returns all recorded method calls.
func (m *MockDevice) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.calls...)
}
