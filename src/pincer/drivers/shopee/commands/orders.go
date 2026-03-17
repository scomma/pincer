package commands

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/shopee"
)

// OrderItem represents one item inside a Shopee order card.
type OrderItem struct {
	Name      string `json:"name"`
	Variation string `json:"variation,omitempty"`
	Quantity  string `json:"quantity"`
	Price     string `json:"price"`
}

// Order represents a single order card on the My Purchases page.
type Order struct {
	Shop   string      `json:"shop"`
	Status string      `json:"status"`
	Items  []OrderItem `json:"items"`
}

// OrderListResult is the output of `shopee order list`.
type OrderListResult struct {
	Orders []Order `json:"orders"`
	Count  int     `json:"count"`
}

// ReorderResult is the output of `shopee order reorder`.
type ReorderResult struct {
	Item    string `json:"item"`
	Shop    string `json:"shop"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// OrderList executes the `shopee order list` command.
// Navigates to My Purchases, scrolls through order cards, and returns parsed orders.
func OrderList(ctx context.Context, driver *shopee.ShopeeDriver, status string, limit int) (*OrderListResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToOrders(ctx); err != nil {
		return nil, fmt.Errorf("navigate to orders: %w", err)
	}

	// If a specific status tab was requested, tap it.
	if status != "" {
		tabText := normalizeStatusTab(status)
		tab, err := driver.Workflow.WaitForElement(ctx, 5*time.Second, core.HasText(tabText))
		if err == nil && tab != nil {
			if err := tab.Tap(ctx, driver.Dev); err != nil {
				return nil, fmt.Errorf("tap %s tab: %w", tabText, err)
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Collect orders across multiple screens by scrolling.
	var orders []Order
	seen := map[string]bool{}
	const maxScrolls = 24
	staleScrolls := 0

	for scroll := 0; scroll <= maxScrolls; scroll++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		finder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		newCount := 0
		for _, order := range parseOrders(finder) {
			key := orderKey(order)
			if !seen[key] {
				seen[key] = true
				orders = append(orders, order)
				newCount++
			}
		}

		if limit > 0 && len(orders) >= limit {
			orders = orders[:limit]
			break
		}

		if newCount == 0 {
			staleScrolls++
			if staleScrolls >= 3 {
				break
			}
		} else {
			staleScrolls = 0
		}

		if scroll < maxScrolls {
			if err := driver.Workflow.ScrollDown(ctx); err != nil {
				return nil, err
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	return &OrderListResult{
		Orders: orders,
		Count:  len(orders),
	}, nil
}

// OrderReorder finds a past order matching itemName and taps "Buy Again".
func OrderReorder(ctx context.Context, driver *shopee.ShopeeDriver, itemName string) (*ReorderResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToOrders(ctx); err != nil {
		return nil, fmt.Errorf("navigate to orders: %w", err)
	}

	// Tap "Completed" tab since Buy Again is available on completed orders.
	tab, err := driver.Workflow.WaitForElement(ctx, 5*time.Second, core.HasText("Completed"))
	if err == nil && tab != nil {
		if err := tab.Tap(ctx, driver.Dev); err != nil {
			return nil, fmt.Errorf("tap Completed tab: %w", err)
		}
		time.Sleep(1 * time.Second)
	}

	// Scroll to find the order containing the matching item.
	lowerName := strings.ToLower(itemName)
	const maxScrolls = 16

	for scroll := 0; scroll <= maxScrolls; scroll++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		finder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		// Find all order shop headers and item sections to locate the matching item.
		shopHeaders := finder.All(core.HasID("viewShopHeader"))
		if len(shopHeaders) == 0 {
			shopHeaders = finder.All(core.HasID("labelShopName"))
		}

		itemSections := finder.All(core.HasID("sectionItemInfo"))

		for _, section := range itemSections {
			name := orderItemName(section)
			if name == "" || !strings.Contains(strings.ToLower(name), lowerName) {
				continue
			}

			// Found the matching item. Determine shop name.
			shopName := nearestShopAbove(shopEntriesFromElements(
				finder.All(core.HasID("labelShopName")),
			), section.Bounds.Top)

			// Find the "Buy Again" button near this order card.
			buyAgainBtn := findBuyAgainNear(finder, section.Bounds.Top, section.Bounds.Bottom, shopHeaders)
			if buyAgainBtn == nil {
				return nil, fmt.Errorf("found item %q but could not locate Buy Again button", itemName)
			}

			// The "Buy Again" text element itself may not be clickable.
			// Walk up to find a clickable parent.
			clickTarget := findClickableAncestor(buyAgainBtn)
			if clickTarget == nil {
				clickTarget = buyAgainBtn
			}

			if err := clickTarget.Tap(ctx, driver.Dev); err != nil {
				return nil, fmt.Errorf("tap Buy Again: %w", err)
			}

			// Wait for the result — could be a product page, variant picker, or direct add.
			time.Sleep(2 * time.Second)

			// Check if a variant/quantity picker appeared and confirm it.
			if err := confirmVariantPickerIfPresent(ctx, driver); err != nil {
				return nil, fmt.Errorf("confirm variant picker: %w", err)
			}

			return &ReorderResult{
				Item:    name,
				Shop:    shopName,
				Success: true,
				Message: "Buy Again tapped successfully",
			}, nil
		}

		if scroll < maxScrolls {
			if err := driver.Workflow.ScrollDown(ctx); err != nil {
				return nil, err
			}
			time.Sleep(250 * time.Millisecond)
		}
	}

	return nil, fmt.Errorf("order item matching %q not found", itemName)
}

// parseOrders extracts Order structs from a UI dump of the My Purchases page.
func parseOrders(finder *core.ElementFinder) []Order {
	var orders []Order

	shopHeaders := finder.All(core.HasID("viewShopHeader"))
	if len(shopHeaders) == 0 {
		return nil
	}

	for _, header := range shopHeaders {
		order := Order{}

		// Extract shop name from labelShopName inside or near the header.
		shopNameEl := firstDescendant(header, func(e *core.Element) bool {
			return e.ResourceID == "labelShopName" && strings.TrimSpace(e.Text) != ""
		})
		if shopNameEl != nil {
			order.Shop = strings.TrimSpace(shopNameEl.Text)
		} else {
			// Fallback: look for labelShopName near the header's Y position.
			allShopNames := finder.All(core.HasID("labelShopName"))
			for _, sn := range allShopNames {
				if abs(sn.Bounds.Top-header.Bounds.Top) < 80 {
					order.Shop = strings.TrimSpace(sn.Text)
					break
				}
			}
		}

		// Extract order status.
		statusEl := firstDescendant(header, func(e *core.Element) bool {
			return e.ResourceID == "labelOrderStatus" && strings.TrimSpace(e.Text) != ""
		})
		if statusEl != nil {
			order.Status = strings.TrimSpace(statusEl.Text)
		} else {
			// Fallback: look for labelOrderStatus near header.
			allStatuses := finder.All(core.HasID("labelOrderStatus"))
			for _, s := range allStatuses {
				if abs(s.Bounds.Top-header.Bounds.Top) < 80 {
					order.Status = strings.TrimSpace(s.Text)
					break
				}
			}
		}

		// Find item sections that belong to this order.
		// Items belong to an order if they are below the header and above the next header.
		headerBottom := header.Bounds.Bottom
		nextHeaderTop := 999999
		for _, other := range shopHeaders {
			if other.Bounds.Top > header.Bounds.Top && other.Bounds.Top < nextHeaderTop {
				nextHeaderTop = other.Bounds.Top
			}
		}

		itemSections := finder.All(core.HasID("sectionItemInfo"))
		for _, section := range itemSections {
			if section.Bounds.Top >= headerBottom && section.Bounds.Top < nextHeaderTop {
				item := parseOrderItem(section)
				if item.Name != "" {
					order.Items = append(order.Items, item)
				}
			}
		}

		if order.Shop != "" || len(order.Items) > 0 {
			orders = append(orders, order)
		}
	}

	return orders
}

// parseOrderItem extracts an OrderItem from a sectionItemInfo element.
func parseOrderItem(section *core.Element) OrderItem {
	item := OrderItem{}

	item.Name = orderItemName(section)

	// Extract variation — second text child or element with variation-like text.
	item.Variation = orderItemVariation(section, item.Name)

	// Extract quantity from labelItemQty.
	qtyEl := firstDescendant(section, func(e *core.Element) bool {
		return e.ResourceID == "labelItemQty" && strings.TrimSpace(e.Text) != ""
	})
	if qtyEl != nil {
		item.Quantity = strings.TrimSpace(qtyEl.Text)
	}

	// Extract price from labelItemPrice.
	priceEl := firstDescendant(section, func(e *core.Element) bool {
		return e.ResourceID == "labelItemPrice" && strings.TrimSpace(e.Text) != ""
	})
	if priceEl != nil {
		item.Price = strings.TrimSpace(priceEl.Text)
	} else {
		// Fallback: find any price-like text.
		walkDescendants(section, func(child *core.Element) {
			if item.Price == "" && child.Text != "" && strings.HasPrefix(child.Text, "฿") &&
				child.ResourceID != "labelItemPriceBeforeDiscount" {
				item.Price = strings.TrimSpace(child.Text)
			}
		})
	}

	return item
}

// orderItemName extracts the product name from a sectionItemInfo element.
// The item name is the first text child with a long product-name-like text
// (no resource ID dedicated to it).
func orderItemName(section *core.Element) string {
	best := ""
	bestScore := -1
	walkDescendants(section, func(child *core.Element) {
		text := strings.TrimSpace(child.Text)
		if score := orderNameScore(text, child.ResourceID); score > bestScore {
			best = text
			bestScore = score
		}
	})
	return best
}

// orderNameScore scores a text string for likelihood of being a product name.
func orderNameScore(text, resourceID string) int {
	switch {
	case text == "":
		return -1
	case strings.HasPrefix(text, "฿"):
		return -1
	case strings.HasPrefix(text, "x") && len(text) <= 4:
		// Quantity like "x1", "x2".
		return -1
	case resourceID == "labelShopName":
		return -1
	case resourceID == "labelOrderStatus":
		return -1
	case resourceID == "labelItemQty":
		return -1
	case resourceID == "labelItemPrice":
		return -1
	case resourceID == "labelItemPriceBeforeDiscount":
		return -1
	case resourceID == "imageItem":
		return -1
	case strings.EqualFold(text, "Buy Again"):
		return -1
	case strings.EqualFold(text, "Rate"):
		return -1
	case strings.EqualFold(text, "Return/Refund"):
		return -1
	}

	score := utf8.RuneCountInString(text)
	if strings.Contains(text, "  ") {
		score -= 2
	}
	return score
}

// orderItemVariation extracts the variation text from a sectionItemInfo element.
// It looks for the second-longest text child that is not the item name.
func orderItemVariation(section *core.Element, itemName string) string {
	var candidates []string
	walkDescendants(section, func(child *core.Element) {
		text := strings.TrimSpace(child.Text)
		if text == "" || text == itemName {
			return
		}
		if strings.HasPrefix(text, "฿") {
			return
		}
		if child.ResourceID == "labelItemQty" || child.ResourceID == "labelItemPrice" ||
			child.ResourceID == "labelItemPriceBeforeDiscount" || child.ResourceID == "imageItem" {
			return
		}
		if strings.EqualFold(text, "Buy Again") || strings.EqualFold(text, "Rate") ||
			strings.EqualFold(text, "Return/Refund") {
			return
		}
		// Quantity pattern like "x1".
		if strings.HasPrefix(text, "x") && len(text) <= 4 {
			return
		}
		candidates = append(candidates, text)
	})

	// The variation is typically the shorter descriptive text (not the long product name).
	for _, c := range candidates {
		if utf8.RuneCountInString(c) < utf8.RuneCountInString(itemName) {
			return c
		}
	}
	return ""
}

// findBuyAgainNear finds the "Buy Again" text element near a given Y range.
func findBuyAgainNear(finder *core.ElementFinder, itemTop, itemBottom int, shopHeaders []*core.Element) *core.Element {
	// Determine the order card bounds: from the nearest shop header above
	// the item to the next shop header below.
	cardTop := 0
	cardBottom := 999999
	for _, h := range shopHeaders {
		if h.Bounds.Top <= itemTop && h.Bounds.Top > cardTop {
			cardTop = h.Bounds.Top
		}
		if h.Bounds.Top > itemBottom && h.Bounds.Top < cardBottom {
			cardBottom = h.Bounds.Top
		}
	}

	// Expand the search range a bit below the item to catch the button row.
	searchBottom := cardBottom
	if searchBottom == 999999 {
		searchBottom = itemBottom + 400
	}

	buyAgains := finder.All(core.HasText("Buy Again"))
	for _, btn := range buyAgains {
		if btn.Bounds.Top >= cardTop && btn.Bounds.Top < searchBottom {
			return btn
		}
	}
	return nil
}

// findClickableAncestor walks up the element tree to find a clickable parent.
func findClickableAncestor(e *core.Element) *core.Element {
	current := e.Parent
	for current != nil {
		if current.Clickable {
			return current
		}
		current = current.Parent
	}
	return nil
}

// confirmVariantPickerIfPresent checks if a variant/quantity picker dialog
// appeared after tapping Buy Again and confirms the selection.
func confirmVariantPickerIfPresent(ctx context.Context, driver *shopee.ShopeeDriver) error {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil // Non-fatal, the tap might have succeeded already.
	}

	// Look for common confirmation buttons in variant/quantity pickers.
	for _, text := range []string{"Add to Cart", "OK", "Confirm"} {
		btn := finder.ByText(text, false)
		if btn != nil && btn.Clickable {
			return btn.Tap(ctx, driver.Dev)
		}
		if btn != nil {
			parent := findClickableAncestor(btn)
			if parent != nil {
				return parent.Tap(ctx, driver.Dev)
			}
		}
	}

	return nil
}

// orderKey generates a deduplication key for an order.
func orderKey(o Order) string {
	var parts []string
	parts = append(parts, o.Shop, o.Status)
	for _, item := range o.Items {
		parts = append(parts, item.Name, item.Price, item.Quantity)
	}
	return strings.Join(parts, "|")
}

// normalizeStatusTab maps user-friendly status names to the tab text on screen.
func normalizeStatusTab(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		return "Completed"
	case "to ship", "toship":
		return "To Ship"
	case "to receive", "toreceive":
		return "To Receive"
	case "return", "refund", "return/refund":
		return "Return/Refund"
	case "cancelled":
		return "Cancelled"
	default:
		return status
	}
}

func shopEntriesFromElements(elements []*core.Element) []shopEntry {
	var entries []shopEntry
	for _, e := range elements {
		entries = append(entries, shopEntry{name: e.Text, y: e.Bounds.Top})
	}
	return entries
}

// ParseOrdersFromXML is a test helper.
func ParseOrdersFromXML(xmlData []byte) ([]Order, error) {
	finder, err := core.NewElementFinderFromXML(xmlData)
	if err != nil {
		return nil, err
	}
	return parseOrders(finder), nil
}
