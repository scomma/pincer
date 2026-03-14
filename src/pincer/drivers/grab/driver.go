package grab

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
)

const (
	PackageName = "com.grabtaxi.passenger"
	AppTimeout  = 10 * time.Second
)

// Screen identifiers for the Grab app.
type Screen string

const (
	ScreenHome        Screen = "HOME"
	ScreenFoodHome    Screen = "FOOD_HOME"
	ScreenFoodResults Screen = "FOOD_RESULTS"
	ScreenRestaurant  Screen = "RESTAURANT"
	ScreenLoginPhone  Screen = "LOGIN_PHONE"
	ScreenLoginOTP    Screen = "LOGIN_OTP"
	ScreenLoginPIN    Screen = "LOGIN_PIN"
	ScreenUnknown     Screen = "UNKNOWN"
)

// GrabDriver implements the Driver interface for the Grab app.
type GrabDriver struct {
	Dev      core.Device
	Workflow *core.Workflow
	Cache    *core.Cache
}

// NewGrabDriver creates a new GrabDriver.
func NewGrabDriver(dev core.Device) (*GrabDriver, error) {
	cache, err := core.NewCache("grab")
	if err != nil {
		return nil, err
	}
	return &GrabDriver{
		Dev:      dev,
		Workflow: core.NewWorkflow(dev),
		Cache:    cache,
	}, nil
}

func (b *GrabDriver) PackageName() string {
	return PackageName
}

func (b *GrabDriver) EnsureAppRunning(ctx context.Context) error {
	return b.Workflow.EnsureApp(ctx, PackageName, AppTimeout)
}

func (b *GrabDriver) EnsureLoggedIn(ctx context.Context) (bool, error) {
	finder, err := b.Workflow.FreshDump(ctx)
	if err != nil {
		return false, err
	}
	screen := DetectScreen(finder)
	switch screen {
	case ScreenLoginPhone, ScreenLoginOTP, ScreenLoginPIN:
		return false, nil
	default:
		return true, nil
	}
}

// DetectScreen determines which Grab screen is currently displayed.
func DetectScreen(finder *core.ElementFinder) Screen {
	// Check for login screens first
	if finder.ByText("Continue With Mobile Number", true) != nil {
		return ScreenLoginPhone
	}
	if finder.ByText("Enter the 6-digit code", false) != nil {
		return ScreenLoginOTP
	}
	if finder.ByText("Enter your PIN", false) != nil {
		return ScreenLoginPIN
	}

	// Grab's food home shows service tiles AND food content (search bar, restaurant cards).
	// The "pure home" (before tapping Food) doesn't have the search bar or feed.
	hasSearchBar := finder.ByID("com.grabtaxi.passenger:id/search_bar_clickable_area") != nil
	hasFoodTile := finder.First(core.HasContentDesc("Food, double tap to select")) != nil
	hasTransportTile := finder.First(core.HasContentDesc("Transport, double tap to select")) != nil

	if hasSearchBar {
		if finder.ByID("com.grabtaxi.passenger:id/duxton_card") != nil {
			return ScreenFoodResults
		}
		return ScreenFoodHome
	}

	// Home screen: has service tiles but no food search bar
	if hasFoodTile && hasTransportTile {
		return ScreenHome
	}

	return ScreenUnknown
}

// NavigateToFoodHome navigates from wherever we are to the Food home screen.
func (b *GrabDriver) NavigateToFoodHome(ctx context.Context) error {
	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		finder, err := b.Workflow.FreshDump(ctx)
		if err != nil {
			// Transient ADB error — retry after a brief pause.
			time.Sleep(1 * time.Second)
			continue
		}

		screen := DetectScreen(finder)
		switch screen {
		case ScreenFoodHome, ScreenFoodResults:
			return nil
		case ScreenHome:
			foodTile := finder.First(core.HasContentDesc("Food, double tap to select"))
			if foodTile == nil {
				foodTile = finder.First(core.HasText("Food"), core.IsClickable())
			}
			if foodTile == nil {
				return core.ErrElementNotFound()
			}
			c := foodTile.Center()
			if err := b.Dev.Tap(ctx, c.X, c.Y); err != nil {
				return err
			}
			_, err := b.Workflow.WaitForElement(ctx, 5*time.Second,
				core.HasID("com.grabtaxi.passenger:id/search_bar_clickable_area"))
			return err
		default:
			if err := b.EnsureAppRunning(ctx); err != nil {
				return err
			}
			time.Sleep(2 * time.Second)
		}
	}
	return core.ErrNavigation()
}
