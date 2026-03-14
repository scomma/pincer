package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/drivers/shopee"
	"github.com/prathan/pincer/src/pincer/core"
)

// Product represents a Shopee search result.
type Product struct {
	Name     string `json:"name"`
	Price    string `json:"price"`
	Discount string `json:"discount,omitempty"`
	Sold     string `json:"sold,omitempty"`
}

// SearchResult is the output of `shopee search`.
type SearchResult struct {
	Products []Product `json:"products"`
	Query    string    `json:"query"`
}

// Search executes the `shopee search` command.
func Search(ctx context.Context, driver *shopee.ShopeeDriver, query string) (*SearchResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	// Tap the search bar on home
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	// Look for search bar — Shopee uses a search input at the top
	searchBar := finder.ByID("com.shopee.th:id/home_square_root")
	if searchBar == nil {
		// Try the text-based search hint
		searchBar = finder.First(func(e *core.Element) bool {
			return e.Hint != "" && e.Class == "android.widget.EditText"
		})
	}
	if searchBar == nil {
		return nil, core.NewDriverError("search_not_found", "Could not find search bar")
	}

	c := searchBar.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}
	time.Sleep(1 * time.Second)

	if err := driver.Dev.TypeText(ctx, query); err != nil {
		return nil, err
	}
	if err := driver.Dev.KeyEvent(ctx, "KEYCODE_ENTER"); err != nil {
		return nil, err
	}
	time.Sleep(2 * time.Second)

	finder, err = driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	products := parseSearchResults(finder)

	return &SearchResult{
		Products: products,
		Query:    query,
	}, nil
}

func parseSearchResults(finder *core.ElementFinder) []Product {
	// Shopee search results are product cards with name, price, discount, sold count
	// This is a best-effort parser for the general structure
	var products []Product

	// Find all elements that look like product names (long text in TextViews)
	allText := finder.All(func(e *core.Element) bool {
		return e.Class == "android.widget.TextView" && len(e.Text) > 20
	})

	for _, el := range allText {
		p := Product{Name: el.Text}
		// Look for price/discount near this element (below it)
		if el.Parent != nil {
			for _, sibling := range el.Parent.Children {
				if sibling.Text != "" && len(sibling.Text) < 20 {
					if strings.HasPrefix(sibling.Text, "฿") {
						p.Price = sibling.Text
					}
					if strings.HasPrefix(sibling.Text, "-") {
						p.Discount = sibling.Text
					}
				}
			}
		}
		products = append(products, p)
	}

	return products
}
