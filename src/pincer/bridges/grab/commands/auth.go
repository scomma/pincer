package commands

import (
	"context"
	"fmt"

	"github.com/prathan/pincer/src/pincer/bridges/grab"
)

// AuthStatusResult is the output of `grab auth status`.
type AuthStatusResult struct {
	LoggedIn bool   `json:"logged_in"`
	Screen   string `json:"screen"`
}

// AuthStatus checks if the user is logged in to Grab.
func AuthStatus(ctx context.Context, bridge *grab.GrabBridge) (*AuthStatusResult, error) {
	if err := bridge.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	// Single dump — detect screen and derive login status from it.
	finder, err := bridge.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	screen := grab.DetectScreen(finder)
	loggedIn := screen != grab.ScreenLoginPhone &&
		screen != grab.ScreenLoginOTP &&
		screen != grab.ScreenLoginPIN

	return &AuthStatusResult{
		LoggedIn: loggedIn,
		Screen:   string(screen),
	}, nil
}
