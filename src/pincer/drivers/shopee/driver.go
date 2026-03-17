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
	// My Purchases as a page title (in the header, y < 300) means we're
	// on the orders page. The Me page also has "My Purchases" text but
	// as a section label further down.
	if mp := finder.ByText("My Purchases", true); mp != nil && mp.Bounds.Top < 300 {
		return ScreenOrders
	}
	if finder.ByID("com.shopee.th:id/homepage_main_recycler") != nil {
		return ScreenHome
	}
	if finder.ByText("Edit Profile", true) != nil || finder.ByID("labelUserName") != nil {
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

		if err := b.Workflow.BackOrRelaunch(ctx, PackageName); err != nil {
			return err
		}
	}
	return core.ErrNavigation()
}

// NavigateToOrders navigates to the My Purchases page.
func (b *ShopeeDriver) NavigateToOrders(ctx context.Context) error {
	const maxRetries = 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		finder, err := b.Workflow.FreshDump(ctx)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		screen := DetectScreen(finder)
		if screen == ScreenOrders {
			return nil
		}

		// If we're on the Me page, look for "View Purchase History".
		if screen == ScreenMe {
			historyBtn := finder.ByText("View Purchase History", false)
			if historyBtn != nil {
				c := historyBtn.Center()
				if err := b.Dev.Tap(ctx, c.X, c.Y); err != nil {
					return err
				}
				_, err := b.Workflow.WaitForElement(ctx, 5*time.Second,
					core.HasText("My Purchases"))
				if err == nil {
					return nil
				}
				continue
			}
			// Scroll down to find "View Purchase History" on the Me page.
			if err := b.Workflow.ScrollDown(ctx); err != nil {
				return err
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Navigate to Me tab first.
		meTab := finder.First(core.HasContentDesc("tab_bar_button_me"))
		if meTab != nil {
			c := meTab.Center()
			if err := b.Dev.Tap(ctx, c.X, c.Y); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if err := b.Workflow.BackOrRelaunch(ctx, PackageName); err != nil {
			return err
		}
	}
	return core.ErrNavigation()
}

// NavigateToHome navigates to the Shopee home screen.
func (b *ShopeeDriver) NavigateToHome(ctx context.Context) error {
	const maxRetries = 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		finder, err := b.Workflow.FreshDump(ctx)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if DetectScreen(finder) == ScreenHome {
			return nil
		}

		// Try tapping the bottom-nav home tab directly — more reliable
		// than pressing back from screens like CART or ME.
		homeTab := finder.First(core.HasContentDesc("tab_bar_button_home"))
		if homeTab != nil {
			c := homeTab.Center()
			if err := b.Dev.Tap(ctx, c.X, c.Y); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
			continue
		}

		if err := b.Workflow.BackOrRelaunch(ctx, PackageName); err != nil {
			return err
		}
	}
	return core.ErrNavigation()
}
