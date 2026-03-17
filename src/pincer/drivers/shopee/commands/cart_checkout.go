package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/shopee"
)

// CheckoutQuotation is the output of `shopee cart checkout`.
type CheckoutQuotation struct {
	Items           []CheckoutItem `json:"items"`
	Subtotal        string         `json:"subtotal"`
	Shipping        string         `json:"shipping"`
	VoucherDiscount string         `json:"voucher_discount,omitempty"`
	Total           string         `json:"total"`
}

// CheckoutItem represents a single item on the checkout page.
type CheckoutItem struct {
	Name     string `json:"name"`
	Price    string `json:"price,omitempty"`
	Quantity string `json:"quantity,omitempty"`
	Shop     string `json:"shop,omitempty"`
}

// CartCheckout selects all cart items, proceeds to the checkout page,
// parses the quotation, and returns it as JSON.
func CartCheckout(ctx context.Context, driver *shopee.ShopeeDriver) (*CheckoutQuotation, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToCart(ctx); err != nil {
		return nil, fmt.Errorf("navigate to cart: %w", err)
	}

	// Scroll to top so the cart items and bottom bar are visible.
	for i := 0; i < 3; i++ {
		if err := driver.Workflow.ScrollUp(ctx); err != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Wait for cart items to load.
	_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, func(e *core.Element) bool {
		return strings.HasPrefix(e.ResourceID, "sectionItemRow_") || e.ResourceID == "labelItemName"
	})

	// Select all items by tapping the "All" checkbox at the bottom bar.
	if err := selectAllItems(ctx, driver); err != nil {
		return nil, fmt.Errorf("select all items: %w", err)
	}

	// Read the subtotal from the cart page.
	time.Sleep(500 * time.Millisecond)
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, fmt.Errorf("read cart subtotal: %w", err)
	}

	// Tap the Checkout button to proceed.
	checkoutBtn := finder.First(core.HasID("buttonCheckout"))
	if checkoutBtn == nil {
		return nil, fmt.Errorf("buttonCheckout not found on cart page")
	}
	if err := checkoutBtn.Tap(ctx, driver.Dev); err != nil {
		return nil, fmt.Errorf("tap checkout button: %w", err)
	}

	// Wait for the checkout page to load.
	_, err = driver.Workflow.WaitForElement(ctx, 8*time.Second, func(e *core.Element) bool {
		return e.ResourceID == "checkoutTotalBottom" || e.ResourceID == "labelTotalPayment"
	})
	if err != nil {
		// Checkout page didn't load -- press back and return error.
		_ = driver.Dev.KeyEvent(ctx, "KEYCODE_BACK")
		return nil, fmt.Errorf("checkout page did not load: %w", err)
	}

	// From this point, ensure we ALWAYS press BACK to exit checkout,
	// even on error.
	quotation, parseErr := parseCheckoutPage(ctx, driver)

	// Exit the checkout page by pressing back.
	backErr := pressCheckoutBack(ctx, driver)

	if parseErr != nil {
		return nil, fmt.Errorf("parse checkout page: %w", parseErr)
	}
	// back-press failure is secondary; we still return the quotation.
	_ = backErr

	return quotation, nil
}

// selectAllItems taps the "All" checkbox at the bottom bar and verifies
// that at least one item is selected.
func selectAllItems(ctx context.Context, driver *shopee.ShopeeDriver) error {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}

	// Find the "All" checkbox wrapper at the bottom bar.
	allCheckbox := finder.First(core.HasID("checkboxTouchableWrapper"))
	if allCheckbox == nil {
		return fmt.Errorf("checkboxTouchableWrapper (All) not found")
	}

	// Check if items are already all selected by looking at checkout button text.
	checkoutLabel := finder.First(core.HasID("labelButtonCheckout"))
	alreadySelected := false
	if checkoutLabel != nil && !strings.Contains(checkoutLabel.Text, "(0)") &&
		strings.Contains(checkoutLabel.Text, "(") {
		alreadySelected = true
	}

	if !alreadySelected {
		if err := allCheckbox.Tap(ctx, driver.Dev); err != nil {
			return fmt.Errorf("tap All checkbox: %w", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Verify selection: checkout text should show Check Out (N) with N > 0.
		finder, err = driver.Workflow.FreshDump(ctx)
		if err != nil {
			return err
		}
		checkoutLabel = finder.First(core.HasID("labelButtonCheckout"))
		if checkoutLabel != nil && strings.Contains(checkoutLabel.Text, "(0)") {
			return fmt.Errorf("no items selected after tapping All checkbox")
		}
	}

	return nil
}

// parseCheckoutPage reads the checkout/order-summary page and builds
// the quotation. It scrolls once to find additional elements.
func parseCheckoutPage(ctx context.Context, driver *shopee.ShopeeDriver) (*CheckoutQuotation, error) {
	q := &CheckoutQuotation{}

	// Parse visible elements, then scroll once and parse again.
	for pass := 0; pass < 2; pass++ {
		if pass == 1 {
			if err := driver.Workflow.ScrollDown(ctx); err != nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		finder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		// Grand total.
		if q.Total == "" {
			if elem := finder.First(core.HasID("labelTotalPayment")); elem != nil {
				q.Total = strings.TrimSpace(elem.Text)
			}
		}

		// Shipping fees -- collect all visible ones.
		shippingElems := finder.All(core.HasID("labelShippingFinalPrice"))
		for _, s := range shippingElems {
			text := strings.TrimSpace(s.Text)
			if text != "" && q.Shipping == "" {
				q.Shipping = text
			}
		}

		// Per-order subtotals.
		orderTotalElems := finder.All(core.HasID("labelOrderTotalPrice"))
		for _, o := range orderTotalElems {
			text := strings.TrimSpace(o.Text)
			if text != "" && q.Subtotal == "" {
				q.Subtotal = text
			}
		}

		// Voucher discounts.
		voucherElems := finder.All(core.HasID("labelPlatformVoucher"))
		for _, v := range voucherElems {
			text := strings.TrimSpace(v.Text)
			if text != "" && q.VoucherDiscount == "" {
				q.VoucherDiscount = text
			}
		}

		// Collect item information from checkout page.
		collectCheckoutItems(finder, q)
	}

	return q, nil
}

// collectCheckoutItems finds item information on the checkout page.
func collectCheckoutItems(finder *core.ElementFinder, q *CheckoutQuotation) {
	// Track already-seen items by name to avoid duplicates across passes.
	seen := make(map[string]bool)
	for _, existing := range q.Items {
		seen[existing.Name] = true
	}

	// Look for item name elements on the checkout page.
	nameElems := finder.All(core.HasID("labelItemName"))
	for _, nameElem := range nameElems {
		name := strings.TrimSpace(nameElem.Text)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		item := CheckoutItem{
			Name: name,
		}

		// Try to find price and shop near this element by walking
		// siblings/parent context. For now, collect names as the
		// checkout page layout varies.
		q.Items = append(q.Items, item)
	}
}

// pressCheckoutBack exits the checkout page by pressing the back button.
func pressCheckoutBack(ctx context.Context, driver *shopee.ShopeeDriver) error {
	// Try the in-app back button first.
	finder, err := driver.Workflow.FreshDump(ctx)
	if err == nil {
		backBtn := finder.First(core.HasID("buttonActionBarBack"))
		if backBtn == nil {
			backBtn = finder.First(core.HasID("buttonActionBarIcon"))
		}
		if backBtn != nil {
			if tapErr := backBtn.Tap(ctx, driver.Dev); tapErr == nil {
				time.Sleep(500 * time.Millisecond)
				return nil
			}
		}
	}

	// Fallback: hardware back key.
	return driver.Dev.KeyEvent(ctx, "KEYCODE_BACK")
}

