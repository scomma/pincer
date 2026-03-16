package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/shopee"
)

// CartUpdateResult is the output of `shopee cart update`.
type CartUpdateResult struct {
	Updated          bool   `json:"updated"`
	Item             string `json:"item"`
	PreviousQuantity int    `json:"previous_quantity"`
	NewQuantity      int    `json:"new_quantity"`
}

// CartUpdate modifies the quantity of a cart item.
func CartUpdate(ctx context.Context, driver *shopee.ShopeeDriver, itemName string, quantity int) (*CartUpdateResult, error) {
	if quantity < 1 {
		return nil, fmt.Errorf("quantity must be at least 1, got %d", quantity)
	}

	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToCart(ctx); err != nil {
		return nil, fmt.Errorf("navigate to cart: %w", err)
	}

	row, _, err := findCartItemRow(ctx, driver, itemName)
	if err != nil {
		return nil, fmt.Errorf("find cart item: %w", err)
	}

	// Read current quantity from the textInputQuantity's child EditText.
	currentQty, err := readQuantity(row)
	if err != nil {
		return nil, fmt.Errorf("read quantity: %w", err)
	}

	// Determine the item's full display name.
	label := firstDescendant(row, func(e *core.Element) bool {
		return e.ResourceID == "labelItemName" && strings.TrimSpace(e.Text) != ""
	})
	fullName := itemName
	if label != nil {
		fullName = strings.TrimSpace(label.Text)
	}

	diff := quantity - currentQty
	if diff == 0 {
		return &CartUpdateResult{
			Updated:          true,
			Item:             fullName,
			PreviousQuantity: currentQty,
			NewQuantity:      currentQty,
		}, nil
	}

	if diff > 0 {
		// Tap the "+" button (diff) times.
		addBtn := firstDescendant(row, func(e *core.Element) bool {
			return e.ResourceID == "buttonAddMoreQuantity"
		})
		if addBtn == nil {
			return nil, fmt.Errorf("buttonAddMoreQuantity not found in item row")
		}
		for i := 0; i < diff; i++ {
			if err := addBtn.Tap(ctx, driver.Dev); err != nil {
				return nil, fmt.Errorf("tap add button: %w", err)
			}
			time.Sleep(300 * time.Millisecond)
		}
	} else {
		// Tap the "-" button (-diff) times.
		reduceBtn := firstDescendant(row, func(e *core.Element) bool {
			return e.ResourceID == "buttonReduceQuantity"
		})
		if reduceBtn == nil {
			return nil, fmt.Errorf("buttonReduceQuantity not found in item row")
		}
		for i := 0; i < -diff; i++ {
			if err := reduceBtn.Tap(ctx, driver.Dev); err != nil {
				return nil, fmt.Errorf("tap reduce button: %w", err)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}

	// Re-dump and verify quantity changed. The item should still be
	// on screen — just re-dump without scrolling.
	time.Sleep(500 * time.Millisecond)
	newQty := quantity // optimistic default
	finder, err := driver.Workflow.FreshDump(ctx)
	if err == nil {
		lowerName := strings.ToLower(fullName)
		rows := finder.All(func(e *core.Element) bool {
			return strings.HasPrefix(e.ResourceID, "sectionItemRow_")
		})
		for _, r := range rows {
			lbl := firstDescendant(r, func(e *core.Element) bool {
				return e.ResourceID == "labelItemName" && strings.TrimSpace(e.Text) != ""
			})
			if lbl != nil && strings.Contains(strings.ToLower(lbl.Text), lowerName) {
				if q, qErr := readQuantity(r); qErr == nil {
					newQty = q
				}
				break
			}
		}
	}

	return &CartUpdateResult{
		Updated:          newQty == quantity,
		Item:             fullName,
		PreviousQuantity: currentQty,
		NewQuantity:      newQty,
	}, nil
}

// readQuantity reads the quantity from a cart item row by finding the
// textInputQuantity element's child EditText.
func readQuantity(row *core.Element) (int, error) {
	// Look for the EditText child inside textInputQuantity.
	var qtyText string
	var found bool

	walkDescendants(row, func(child *core.Element) {
		if found {
			return
		}
		if child.ResourceID == "textInputQuantity" {
			// Find the EditText child.
			for _, grandchild := range child.Children {
				if grandchild.Class == "android.widget.EditText" && grandchild.Text != "" {
					qtyText = strings.TrimSpace(grandchild.Text)
					found = true
					return
				}
			}
		}
	})

	if !found {
		// Fallback: look for any EditText with a numeric value in the row.
		walkDescendants(row, func(child *core.Element) {
			if found {
				return
			}
			if child.Class == "android.widget.EditText" && child.Text != "" {
				qtyText = strings.TrimSpace(child.Text)
				found = true
			}
		})
	}

	if !found {
		return 0, fmt.Errorf("quantity element not found in item row")
	}

	qty, err := strconv.Atoi(qtyText)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity %q: %w", qtyText, err)
	}
	return qty, nil
}
