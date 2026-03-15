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

// CartItem represents an item in the Shopee shopping cart.
type CartItem struct {
	Shop      string `json:"shop"`
	Name      string `json:"name"`
	Variation string `json:"variation,omitempty"`
	Price     string `json:"price"`
	OldPrice  string `json:"old_price,omitempty"`
	Quantity  string `json:"quantity,omitempty"`
}

// CartListResult is the output of `shopee cart list`.
type CartListResult struct {
	Items []CartItem `json:"items"`
	Count int        `json:"count"`
}

type shopEntry struct {
	name string
	y    int
}

// CartList executes the `shopee cart list` command.
// Scrolls down to collect all cart items, not just the first visible page.
func CartList(ctx context.Context, driver *shopee.ShopeeDriver) (*CartListResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToCart(ctx); err != nil {
		return nil, fmt.Errorf("navigate to cart: %w", err)
	}

	// Collect items across multiple screens by scrolling.
	var items []CartItem
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
		for _, item := range parseCartItems(finder) {
			key := item.Shop + "|" + item.Name + "|" + item.Variation + "|" + item.Price
			if !seen[key] {
				seen[key] = true
				items = append(items, item)
				newCount++
			}
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

	return &CartListResult{
		Items: items,
		Count: len(items),
	}, nil
}

func parseCartItems(finder *core.ElementFinder) []CartItem {
	var items []CartItem

	shopNames := finder.All(core.HasID("labelShopName"))
	rowContainers := finder.All(func(e *core.Element) bool {
		return strings.HasPrefix(e.ResourceID, "sectionItemRow_")
	})
	if len(rowContainers) == 0 {
		rowContainers = finder.All(func(e *core.Element) bool {
			return e.ResourceID == "labelItemName"
		})
	}

	var shops []shopEntry
	for _, s := range shopNames {
		shops = append(shops, shopEntry{name: s.Text, y: s.Bounds.Top})
	}

	for _, row := range rowContainers {
		item := CartItem{
			Shop: nearestShopAbove(shops, row.Bounds.Top),
			Name: cartItemName(row),
		}
		if item.Name == "" {
			continue
		}

		populateCartItem(row, &item)
		items = append(items, item)
	}

	return items
}

func nearestShopAbove(shops []shopEntry, y int) string {
	shop := ""
	for _, s := range shops {
		if s.y < y {
			shop = s.name
		}
	}
	return shop
}

func cartItemName(row *core.Element) string {
	if named := firstDescendant(row, func(e *core.Element) bool {
		return e.ResourceID == "labelItemName" && strings.TrimSpace(e.Text) != ""
	}); named != nil {
		return strings.TrimSpace(named.Text)
	}

	best := ""
	bestScore := -1
	walkDescendants(row, func(child *core.Element) {
		text := strings.TrimSpace(child.Text)
		if score := cartNameScore(text, child.ResourceID); score > bestScore {
			best = text
			bestScore = score
		}
	})
	return best
}

func cartNameScore(text, resourceID string) int {
	switch {
	case text == "":
		return -1
	case strings.HasPrefix(text, "฿"):
		return -1
	case resourceID == "labelShopName":
		return -1
	case strings.Contains(strings.ToLower(text), "voucher"):
		return -1
	case strings.EqualFold(text, "Edit"):
		return -1
	case strings.EqualFold(text, "New"):
		return -1
	}

	score := utf8.RuneCountInString(text)
	if strings.Contains(text, "  ") {
		score -= 2
	}
	return score
}

func populateCartItem(row *core.Element, item *CartItem) {
	walkDescendants(row, func(child *core.Element) {
		switch child.ResourceID {
		case "labelVariation":
			if item.Variation == "" {
				item.Variation = strings.TrimSpace(child.Text)
			}
		case "labelPriceBeforeDiscount":
			if item.OldPrice == "" {
				item.OldPrice = strings.TrimSpace(child.Text)
			}
		}

		if child.Text != "" && strings.HasPrefix(child.Text, "฿") && item.Price == "" &&
			child.ResourceID != "labelPriceBeforeDiscount" {
			item.Price = strings.TrimSpace(child.Text)
		}

		if child.Class == "android.widget.EditText" && child.Text != "" && item.Quantity == "" {
			item.Quantity = strings.TrimSpace(child.Text)
		}
	})
}

func firstDescendant(root *core.Element, match func(*core.Element) bool) *core.Element {
	if match(root) {
		return root
	}
	for _, child := range root.Children {
		if found := firstDescendant(child, match); found != nil {
			return found
		}
	}
	return nil
}

func walkDescendants(root *core.Element, visit func(*core.Element)) {
	for _, child := range root.Children {
		visit(child)
		walkDescendants(child, visit)
	}
}

// ParseCartItemsFromXML is a test helper.
func ParseCartItemsFromXML(xmlData []byte) ([]CartItem, error) {
	finder, err := core.NewElementFinderFromXML(xmlData)
	if err != nil {
		return nil, err
	}
	return parseCartItems(finder), nil
}
