package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/prathan/pincer/src/pincer/drivers/shopee"
	"github.com/prathan/pincer/src/pincer/core"
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

// CartList executes the `shopee cart list` command.
func CartList(ctx context.Context, driver *shopee.ShopeeDriver) (*CartListResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToCart(ctx); err != nil {
		return nil, fmt.Errorf("navigate to cart: %w", err)
	}

	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	items := parseCartItems(finder)
	return &CartListResult{
		Items: items,
		Count: len(items),
	}, nil
}

func parseCartItems(finder *core.ElementFinder) []CartItem {
	var items []CartItem
	var currentShop string

	// Shopee uses bare resource IDs (no package prefix)
	itemNames := finder.All(core.HasID("labelItemName"))
	shopNames := finder.All(core.HasID("labelShopName"))

	// Build a map of shop names by vertical position for association
	type shopEntry struct {
		name string
		y    int
	}
	var shops []shopEntry
	for _, s := range shopNames {
		shops = append(shops, shopEntry{name: s.Text, y: s.Bounds.Top})
	}

	for _, itemEl := range itemNames {
		// Find the nearest shop above this item
		currentShop = ""
		for _, s := range shops {
			if s.y < itemEl.Bounds.Top {
				currentShop = s.name
			}
		}

		item := CartItem{
			Shop: currentShop,
			Name: itemEl.Text,
		}

		// Look for sibling variation and price elements near this item
		if itemEl.Parent != nil {
			walkSiblings(itemEl.Parent, &item)
		}

		items = append(items, item)
	}

	return items
}

func walkSiblings(parent *core.Element, item *CartItem) {
	for _, child := range parent.Children {
		switch child.ResourceID {
		case "labelVariation":
			item.Variation = child.Text
		case "labelPriceBeforeDiscount":
			item.OldPrice = child.Text
		}

		if child.Text != "" && strings.HasPrefix(child.Text, "฿") && item.Price == "" &&
			child.ResourceID != "labelPriceBeforeDiscount" {
			item.Price = child.Text
		}

		if child.Class == "android.widget.EditText" && child.Text != "" && item.Quantity == "" {
			item.Quantity = child.Text
		}

		walkSiblings(child, item)
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
