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

// ShopeeBridge implements the Bridge interface for the Shopee app.
type ShopeeBridge struct {
	Dev      core.Device
	Workflow *core.Workflow
	Cache    *core.Cache
}

func NewShopeeBridge(dev core.Device) (*ShopeeBridge, error) {
	cache, err := core.NewCache("shopee")
	if err != nil {
		return nil, err
	}
	return &ShopeeBridge{
		Dev:      dev,
		Workflow: core.NewWorkflow(dev),
		Cache:    cache,
	}, nil
}

func (b *ShopeeBridge) PackageName() string {
	return PackageName
}

func (b *ShopeeBridge) EnsureAppRunning(ctx context.Context) error {
	return b.Workflow.EnsureApp(ctx, PackageName, AppTimeout)
}

func (b *ShopeeBridge) EnsureLoggedIn(ctx context.Context) (bool, error) {
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
func (b *ShopeeBridge) NavigateToCart(ctx context.Context) error {
	finder, err := b.Workflow.FreshDump(ctx)
	if err != nil {
		return err
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
		time.Sleep(2 * time.Second)
		return nil
	}

	return core.ErrNavigation()
}
