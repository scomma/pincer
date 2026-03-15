package core

import (
	"context"
	"testing"
	"time"
)

type packageSequenceDevice struct {
	packages []string
	idx      int
	keys     []string
	launches []string
}

func (d *packageSequenceDevice) DumpUI(context.Context) (string, error) { return "", nil }
func (d *packageSequenceDevice) Tap(context.Context, int, int) error    { return nil }
func (d *packageSequenceDevice) TypeText(context.Context, string) error { return nil }
func (d *packageSequenceDevice) ClearField(context.Context) error       { return nil }
func (d *packageSequenceDevice) Swipe(context.Context, int, int, int, int, int) error {
	return nil
}
func (d *packageSequenceDevice) Screenshot(context.Context) ([]byte, error) { return nil, nil }
func (d *packageSequenceDevice) LaunchApp(_ context.Context, pkg string) error {
	d.launches = append(d.launches, pkg)
	return nil
}
func (d *packageSequenceDevice) IsScreenOn(context.Context) (bool, error) { return true, nil }
func (d *packageSequenceDevice) WakeScreen(context.Context) error         { return nil }

func (d *packageSequenceDevice) KeyEvent(_ context.Context, key string) error {
	d.keys = append(d.keys, key)
	return nil
}

func (d *packageSequenceDevice) CurrentPackage(context.Context) (string, error) {
	if len(d.packages) == 0 {
		return "", nil
	}
	if d.idx >= len(d.packages) {
		return d.packages[len(d.packages)-1], nil
	}
	pkg := d.packages[d.idx]
	d.idx++
	return pkg, nil
}

func TestWaitForPackageDismissesCredentialChooser(t *testing.T) {
	dev := &packageSequenceDevice{
		packages: []string{
			"com.google.android.gms",
			"com.grabtaxi.passenger",
		},
	}
	workflow := NewWorkflow(dev)

	if err := workflow.WaitForPackage(context.Background(), "com.grabtaxi.passenger", 5*time.Second); err != nil {
		t.Fatalf("WaitForPackage: %v", err)
	}

	if len(dev.keys) != 1 || dev.keys[0] != "KEYCODE_BACK" {
		t.Fatalf("expected one KEYCODE_BACK, got %v", dev.keys)
	}
	if len(dev.launches) != 1 || dev.launches[0] != "com.grabtaxi.passenger" {
		t.Fatalf("expected relaunch of com.grabtaxi.passenger, got %v", dev.launches)
	}
}
