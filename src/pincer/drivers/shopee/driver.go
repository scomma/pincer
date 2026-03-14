package shopee

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
)

const (
	PackageName = "com.shopee.th"
	AppTimeout  = 10 * time.Second
)

type Screen string

const (
	ScreenHome    Screen = "HOME"
	ScreenCart    Screen = "CART"
	ScreenOrders  Screen = "ORDERS"
	ScreenSearch  Screen = "SEARCH"
	ScreenMe      Screen = "ME"
	ScreenUnknown Screen = "UNKNOWN"
)

// ShopeeDriver implements the Driver interface for the Shopee app.
type ShopeeDriver struct {
	Dev      core.Device
	Workflow *core.Workflow
	Cache    *core.Cache
}

func NewShopeeDriver(dev core.Device) (*ShopeeDriver, error) {
	cache, err := core.NewCache("shopee")
	if err != nil {
		return nil, err
	}
	return &ShopeeDriver{
		Dev:      dev,
		Workflow: core.NewWorkflow(dev),
		Cache:    cache,
	}, nil
}

func (b *ShopeeDriver) PackageName() string {
	return PackageName
}

func (b *ShopeeDriver) EnsureAppRunning(ctx context.Context) error {
	return b.Workflow.EnsureApp(ctx, PackageName, AppTimeout)
}

func (b *ShopeeDriver) EnsureLoggedIn(ctx context.Context) (bool, error) {
	finder, err := b.Workflow.FreshDump(ctx)
	if err != nil {
		return false, err
	}
	if finder.ByID("com.shopee.th:id/homepage_main_recycler") != nil {
		return true, nil
	}
	if finder.First(core.HasID("labelShopName")) != nil {
		return true, nil
	}
	return false, nil
}

// DetectScreen determines which Shopee screen is currently displayed.
func DetectScreen(finder *core.ElementFinder) Screen {
	if finder.ByText("Shopping Cart", true) != nil {
		return ScreenCart
	}
	if finder.ByID("com.shopee.th:id/homepage_main_recycler") != nil {
		return ScreenHome
	}
	if finder.ByText("Edit Profile", true) != nil {
		return ScreenMe
	}
	return ScreenUnknown
}

// NavigateToCart navigates to the shopping cart.
func (b *ShopeeDriver) NavigateToCart(ctx context.Context) error {
	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		finder, err := b.Workflow.FreshDump(ctx)
		if err != nil {
			// Transient ADB error — retry after a brief pause.
			time.Sleep(1 * time.Second)
			continue
		}

		screen := DetectScreen(finder)
		if screen == ScreenCart {
			return nil
		}

		cartBtn := finder.ByID("com.shopee.th:id/cart_btn")
		if cartBtn != nil {
			c := cartBtn.Center()
			if err := b.Dev.Tap(ctx, c.X, c.Y); err != nil {
				return err
			}
			// Wait for the cart screen to appear instead of a fixed sleep.
			_, err := b.Workflow.WaitForElement(ctx, 5*time.Second,
				core.HasText("Shopping Cart"))
			return err
		}

		// Unknown screen — restart app and retry.
		if err := b.EnsureAppRunning(ctx); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
	}
	return core.ErrNavigation()
}

// NavigateToHome navigates to the Shopee home screen.
func (b *ShopeeDriver) NavigateToHome(ctx context.Context) error {
	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		finder, err := b.Workflow.FreshDump(ctx)
		if err != nil {
			// Transient ADB error — retry after a brief pause.
			time.Sleep(1 * time.Second)
			continue
		}

		if DetectScreen(finder) == ScreenHome {
			return nil
		}

		// Try pressing back to return to home.
		if err := b.Dev.KeyEvent(ctx, "KEYCODE_BACK"); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)

		finder, err = b.Workflow.FreshDump(ctx)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		if DetectScreen(finder) == ScreenHome {
			return nil
		}

		// Restart the app as a last resort.
		if err := b.EnsureAppRunning(ctx); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
	}
	return core.ErrNavigation()
}
