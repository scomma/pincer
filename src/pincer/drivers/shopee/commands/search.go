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

	searchBar := finder.ByID("com.shopee.th:id/search_bar")
	if searchBar == nil {
		searchBar = finder.ByID("com.shopee.th:id/search_bar_cell")
	}
	if searchBar == nil {
		searchBar = finder.ByID("com.shopee.th:id/home_square_root")
	}
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

	if err := typeVerifiedShopeeQuery(ctx, driver, query); err != nil {
		return nil, err
	}
	if err := driver.Dev.KeyEvent(ctx, "KEYCODE_ENTER"); err != nil {
		return nil, err
	}

	// Wait for search results to load.
	time.Sleep(1200 * time.Millisecond)

	// Collect products across multiple screens by scrolling.
	var products []Product
	seen := map[string]bool{}
	const maxScrolls = 8
	staleScrolls := 0

	for scroll := 0; scroll <= maxScrolls; scroll++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
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
			staleScrolls++
			if staleScrolls >= 2 {
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

	return &SearchResult{
		Products: products,
		Query:    query,
	}, nil
}

func parseSearchResults(finder *core.ElementFinder) []Product {
	var products []Product
	seenContainers := map[*core.Element]bool{}

	nameEls := finder.All(func(e *core.Element) bool {
		return e.Class == "android.widget.TextView" && looksLikeProductName(e.Text)
	})

	for _, el := range nameEls {
		container := searchResultContainer(el)
		if container != nil {
			if seenContainers[container] {
				continue
			}
			seenContainers[container] = true
		}

		scope := container
		if scope == nil {
			scope = el
		}
		name := bestProductName(scope)
		if name == "" {
			name = strings.TrimSpace(el.Text)
		}

		p := Product{Name: name}
		walkDescendants(scope, func(child *core.Element) {
			text := strings.TrimSpace(child.Text)
			switch {
			case strings.HasPrefix(text, "฿") && p.Price == "":
				p.Price = text
			case strings.HasPrefix(text, "-") && p.Discount == "":
				p.Discount = text
			case looksLikeSoldCount(text) && p.Sold == "":
				p.Sold = text
			}
		})

		if p.Price == "" && container != nil && container.Parent != nil {
			// Some builds render price as a sibling branch above the text node.
			walkDescendants(container.Parent, func(child *core.Element) {
				text := strings.TrimSpace(child.Text)
				if strings.HasPrefix(text, "฿") && p.Price == "" {
					p.Price = text
				}
			})
		}

		products = append(products, p)
	}

	return products
}

func looksLikeProductName(text string) bool {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)

	switch {
	case utf8.RuneCountInString(trimmed) < 12:
		return false
	case strings.HasPrefix(trimmed, "฿"):
		return false
	case strings.HasPrefix(lower, "จังหวัด"):
		return false
	case strings.EqualFold(trimmed, "Express Delivery"):
		return false
	case strings.EqualFold(trimmed, "Shopee Preferred"):
		return false
	case strings.EqualFold(trimmed, "See Original"):
		return false
	case strings.Contains(lower, "shop rating"):
		return false
	case strings.Contains(lower, "สินค้าแนะนำ"):
		return false
	case strings.Contains(lower, "ค้นหายอดนิยม"):
		return false
	case looksLikeSoldCount(trimmed):
		return false
	default:
		return true
	}
}

func bestProductName(scope *core.Element) string {
	best := ""
	bestScore := -1
	walkDescendants(scope, func(child *core.Element) {
		text := strings.TrimSpace(child.Text)
		if score := searchNameScore(text); score > bestScore {
			best = text
			bestScore = score
		}
	})
	return best
}

func searchNameScore(text string) int {
	if !looksLikeProductName(text) {
		return -1
	}

	score := utf8.RuneCountInString(text)
	// Truncated names (ending with "…") tend to be real product titles.
	if strings.Contains(text, "…") {
		score += 8
	}
	// Names with digits are more likely product titles than UI labels.
	if strings.ContainsAny(text, "0123456789") {
		score += 4
	}
	// ALL-CAPS short strings are usually badges or labels, not product names.
	if strings.ToUpper(text) == text && utf8.RuneCountInString(text) < 24 {
		score -= 12
	}
	return score
}

func looksLikeSoldCount(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "sold") || strings.Contains(lower, "ชิ้น")
}

func searchResultContainer(el *core.Element) *core.Element {
	for current := el.Parent; current != nil; current = current.Parent {
		if current.Clickable && current.Height() > 150 && current.Width() > 300 {
			return current
		}
		if current.Scrollable {
			break
		}
	}
	return el.Parent
}

func typeVerifiedShopeeQuery(ctx context.Context, driver *shopee.ShopeeDriver, query string) error {
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := driver.Dev.ClearField(ctx); err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)

		if err := driver.Dev.TypeText(ctx, query); err != nil {
			return err
		}
		time.Sleep(250 * time.Millisecond)

		matches, err := shopeeQueryMatchesInput(ctx, driver, query)
		if err != nil {
			return err
		}
		if matches {
			return nil
		}
	}

	return core.NewDriverError("input_mismatch", "search query did not match the text entered in Shopee")
}

func shopeeQueryMatchesInput(ctx context.Context, driver *shopee.ShopeeDriver, query string) (bool, error) {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return false, err
	}

	current := finder.First(func(e *core.Element) bool {
		return e.Class == "android.widget.EditText" && strings.TrimSpace(e.Text) != ""
	})
	if current == nil {
		// No EditText with visible text — assume the text landed. Many
		// search fields don't expose typed content via accessibility.
		return true, nil
	}

	return strings.EqualFold(strings.TrimSpace(current.Text), strings.TrimSpace(query)), nil
}
