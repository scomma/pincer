package commands

import (
	"context"
	"fmt"

	"github.com/prathan/pincer/src/pincer/drivers/grab"
)

// AuthStatusResult is the output of `grab auth status`.
type AuthStatusResult struct {
	LoggedIn bool   `json:"logged_in"`
	Screen   string `json:"screen"`
}

// AuthStatus checks if the user is logged in to Grab.
func AuthStatus(ctx context.Context, driver *grab.GrabDriver) (*AuthStatusResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	// Single dump — detect screen and derive login status from it.
	finder, err := driver.Workflow.FreshDump(ctx)
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
