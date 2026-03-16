package commands

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/shopee"
)

// CartRemoveResult is the output of `shopee cart remove`.
type CartRemoveResult struct {
	Removed bool   `json:"removed"`
	Item    string `json:"item"`
}

// CartRemove removes an item from the Shopee shopping cart.
// It enters edit mode for the item's shop section, taps the Delete button
// near the item, handles any confirmation dialog, and verifies the item is gone.
func CartRemove(ctx context.Context, driver *shopee.ShopeeDriver, itemName string) (*CartRemoveResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToCart(ctx); err != nil {
		return nil, fmt.Errorf("navigate to cart: %w", err)
	}

	row, finder, err := findCartItemRow(ctx, driver, itemName)
	if err != nil {
		return nil, fmt.Errorf("find cart item: %w", err)
	}

	// Determine the item's full display name.
	label := firstDescendant(row, func(e *core.Element) bool {
		return e.ResourceID == "labelItemName" && strings.TrimSpace(e.Text) != ""
	})
	fullName := itemName
	if label != nil {
		fullName = strings.TrimSpace(label.Text)
	}

	itemY := row.Bounds.Top

	// Find the nearest buttonEdit element ABOVE the item's Y position.
	// This is the per-shop Edit button.
	editBtn := findNearestEditButton(finder, itemY)
	if editBtn == nil {
		return nil, fmt.Errorf("could not find Edit button for item %q", fullName)
	}

	// Tap Edit to enter edit mode.
	if err := editBtn.Tap(ctx, driver.Dev); err != nil {
		return nil, fmt.Errorf("tap edit button: %w", err)
	}

	// Wait for edit mode UI to appear (Delete buttons become visible).
	time.Sleep(800 * time.Millisecond)

	// Find the Delete text element near the item's Y position.
	deleteBtn, err := findDeleteButton(ctx, driver, itemY)
	if err != nil {
		return nil, fmt.Errorf("find delete button: %w", err)
	}

	// Tap the Delete button (or its clickable parent).
	if err := tapClickable(ctx, driver, deleteBtn); err != nil {
		return nil, fmt.Errorf("tap delete button: %w", err)
	}

	// Handle confirmation dialog if it appears.
	time.Sleep(800 * time.Millisecond)
	if err := dismissConfirmationDialog(ctx, driver); err != nil {
		// Non-fatal: dialog may not appear.
		_ = err
	}

	// Wait for the deletion animation/transition.
	time.Sleep(500 * time.Millisecond)

	// If a "Done" button is still visible, tap it to exit edit mode.
	tapDoneIfPresent(ctx, driver)

	// Verify the item is gone.
	time.Sleep(500 * time.Millisecond)
	removed := verifyItemRemoved(ctx, driver, fullName)

	return &CartRemoveResult{
		Removed: removed,
		Item:    fullName,
	}, nil
}

// findNearestEditButton finds the buttonEdit element closest above the given Y position.
func findNearestEditButton(finder *core.ElementFinder, itemY int) *core.Element {
	editButtons := finder.All(func(e *core.Element) bool {
		return e.ResourceID == "buttonEdit"
	})

	var best *core.Element
	bestDist := math.MaxInt32

	for _, btn := range editButtons {
		btnY := btn.Bounds.Top
		// The Edit button should be above or at the same level as the item.
		if btnY <= itemY {
			dist := itemY - btnY
			if dist < bestDist {
				bestDist = dist
				best = btn
			}
		}
	}

	// Fallback: if no edit button is above, pick the closest one overall.
	if best == nil {
		for _, btn := range editButtons {
			dist := abs(btn.Bounds.Top - itemY)
			if dist < bestDist {
				bestDist = dist
				best = btn
			}
		}
	}

	return best
}

// findDeleteButton re-dumps the UI and finds the "Delete" text element
// closest to the item's Y position.
func findDeleteButton(ctx context.Context, driver *shopee.ShopeeDriver, itemY int) (*core.Element, error) {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, fmt.Errorf("fresh dump after edit: %w", err)
	}

	deleteElements := finder.All(func(e *core.Element) bool {
		return strings.EqualFold(strings.TrimSpace(e.Text), "Delete")
	})

	if len(deleteElements) == 0 {
		return nil, fmt.Errorf("no Delete button found on screen")
	}

	// Find the Delete element closest to the item's Y position.
	var best *core.Element
	bestDist := math.MaxInt32

	for _, el := range deleteElements {
		dist := abs(el.Center().Y - itemY)
		if dist < bestDist {
			bestDist = dist
			best = el
		}
	}

	return best, nil
}

// tapClickable taps the element if it's clickable, otherwise walks up to
// find a clickable parent and taps that.
func tapClickable(ctx context.Context, driver *shopee.ShopeeDriver, el *core.Element) error {
	target := el
	for target != nil && !target.Clickable {
		target = target.Parent
	}
	if target == nil {
		// Fall back to tapping the element directly.
		target = el
	}
	return target.Tap(ctx, driver.Dev)
}

// dismissConfirmationDialog looks for a confirmation dialog and taps the
// confirm/OK button if present.
func dismissConfirmationDialog(ctx context.Context, driver *shopee.ShopeeDriver) error {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}

	// Look for common confirmation button texts.
	for _, text := range []string{"Delete", "Confirm", "OK", "Yes", "Remove"} {
		btn := finder.First(func(e *core.Element) bool {
			return strings.EqualFold(strings.TrimSpace(e.Text), text) && e.Clickable
		})
		if btn != nil {
			return btn.Tap(ctx, driver.Dev)
		}
	}

	return nil
}

// tapDoneIfPresent finds a "Done" button (the edit mode exit button) and taps it.
func tapDoneIfPresent(ctx context.Context, driver *shopee.ShopeeDriver) {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return
	}

	// The Done button has the same resource-id as Edit (buttonEdit) but text "Done".
	done := finder.First(func(e *core.Element) bool {
		return e.ResourceID == "buttonEdit" && strings.EqualFold(strings.TrimSpace(e.Text), "Done")
	})
	if done != nil {
		_ = done.Tap(ctx, driver.Dev)
		time.Sleep(500 * time.Millisecond)
	}
}

// verifyItemRemoved re-dumps the UI and checks that the item is no longer visible.
func verifyItemRemoved(ctx context.Context, driver *shopee.ShopeeDriver, fullName string) bool {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return false
	}

	lowerName := strings.ToLower(fullName)
	rows := finder.All(func(e *core.Element) bool {
		return strings.HasPrefix(e.ResourceID, "sectionItemRow_")
	})

	for _, row := range rows {
		lbl := firstDescendant(row, func(e *core.Element) bool {
			return e.ResourceID == "labelItemName" && strings.TrimSpace(e.Text) != ""
		})
		if lbl != nil && strings.Contains(strings.ToLower(lbl.Text), lowerName) {
			return false
		}
	}

	return true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
