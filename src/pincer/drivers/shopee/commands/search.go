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
// Navigates to home, searches, then scrolls to collect results.
func Search(ctx context.Context, driver *shopee.ShopeeDriver, query string) (*SearchResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToHome(ctx); err != nil {
		return nil, fmt.Errorf("navigate to home: %w", err)
	}

	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	searchBar := finder.ByID("com.shopee.th:id/home_square_root")
	if searchBar == nil {
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

	// Wait for the search input to be ready.
	_, _ = driver.Workflow.WaitForElement(ctx, 3*time.Second, func(e *core.Element) bool {
		return e.Focused && e.Class == "android.widget.EditText"
	})

	if err := driver.Dev.TypeText(ctx, query); err != nil {
		return nil, err
	}
	if err := driver.Dev.KeyEvent(ctx, "KEYCODE_ENTER"); err != nil {
		return nil, err
	}

	// Wait for search results to load.
	time.Sleep(2 * time.Second)

	// Collect products across multiple screens by scrolling.
	var products []Product
	seen := map[string]bool{}
	const maxScrolls = 5

	for scroll := 0; scroll <= maxScrolls; scroll++ {
		finder, err = driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		newCount := 0
		for _, p := range parseSearchResults(finder) {
			if !seen[p.Name] {
				seen[p.Name] = true
				products = append(products, p)
				newCount++
			}
		}

		if newCount == 0 {
			break
		}

		if scroll < maxScrolls {
			if err := driver.Dev.Swipe(ctx, 540, 1600, 540, 800, 300); err != nil {
				return nil, err
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

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
