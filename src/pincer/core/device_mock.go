package core

import (
	"context"
	"fmt"
	"os"
	"sync"
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
}

// NewMockDevice creates a MockDevice with a single dump fixture.
func NewMockDevice(dumpFixturePath string, currentPkg string) *MockDevice {
	return &MockDevice{
		pkg:   currentPkg,
		dumps: []string{dumpFixturePath},
	}
}

// NewMockDeviceWithSequence creates a MockDevice that cycles through fixtures.
func NewMockDeviceWithSequence(fixtures []string, currentPkg string) *MockDevice {
	return &MockDevice{
		pkg:   currentPkg,
		dumps: fixtures,
	}
}

func (m *MockDevice) DumpUI(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, "DumpUI")
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

// SetDump changes the fixture(s) returned by DumpUI.
func (m *MockDevice) SetDump(paths ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dumps = paths
	m.dumpIdx = 0
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
