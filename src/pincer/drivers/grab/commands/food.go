package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/drivers/grab"
	"github.com/prathan/pincer/src/pincer/core"
)

// Restaurant represents a parsed restaurant from the food listing.
type Restaurant struct {
	Name  string `json:"name"`
	Promo string `json:"promo,omitempty"`
}

// FoodSearchResult is the output of the food search command.
type FoodSearchResult struct {
	Restaurants []Restaurant `json:"restaurants"`
	Query       string       `json:"query,omitempty"`
}

// FoodSearch executes the `grab food search` command.
// It navigates to the food home, optionally searches, and parses restaurant cards.
// Scrolls down to collect restaurants beyond the first visible page.
func FoodSearch(ctx context.Context, driver *grab.GrabDriver, query string) (*FoodSearchResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToFoodHome(ctx); err != nil {
		return nil, fmt.Errorf("navigate to food home: %w", err)
	}

	if query != "" {
		if err := performSearch(ctx, driver, query); err != nil {
			return nil, err
		}
		// Wait for search results to load.
		_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, func(e *core.Element) bool {
			return e.ResourceID == "com.grabtaxi.passenger:id/duxton_card" ||
				e.ResourceID == "com.grabtaxi.passenger:id/horizontal_merchant_card"
		})
	}

	// Collect restaurants across multiple screens by scrolling.
	var restaurants []Restaurant
	seen := map[string]bool{}
	const maxScrolls = 5

	for scroll := 0; scroll <= maxScrolls; scroll++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		finder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		newCount := 0
		for _, r := range parseRestaurantCards(finder) {
			if !seen[r.Name] {
				seen[r.Name] = true
				restaurants = append(restaurants, r)
				newCount++
			}
		}

		if newCount == 0 {
			break
		}

		if scroll < maxScrolls {
			if err := driver.Workflow.ScrollDown(ctx); err != nil {
				return nil, err
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	return &FoodSearchResult{
		Restaurants: restaurants,
		Query:       query,
	}, nil
}

func findSearchBar(ctx context.Context, driver *grab.GrabDriver) (*core.Element, error) {
	// The search bar hides when scrolled down. Scroll up to reveal it.
	for i := 0; i < 4; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		finder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		bar := finder.ByID("com.grabtaxi.passenger:id/search_bar_clickable_area")
		if bar == nil {
			bar = finder.ByID("com.grabtaxi.passenger:id/search_entry_content")
		}
		if bar != nil {
			return bar, nil
		}

		if err := driver.Workflow.ScrollUp(ctx); err != nil {
			return nil, err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, core.ErrElementNotFound()
}

func performSearch(ctx context.Context, driver *grab.GrabDriver, query string) error {
	searchBar, err := findSearchBar(ctx, driver)
	if err != nil {
		return err
	}

	c := searchBar.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return err
	}

	// Wait for the search input field to be ready.
	_, _ = driver.Workflow.WaitForElement(ctx, 3*time.Second, func(e *core.Element) bool {
		return e.Focused && e.Class == "android.widget.EditText"
	})

	// Clear any leftover text from previous searches.
	if err := driver.Dev.ClearField(ctx); err != nil {
		return err
	}
	// Pause to let the input system process all the delete keyevents
	// before typing new text. Without this, characters interleave.
	time.Sleep(1 * time.Second)

	if err := driver.Dev.TypeText(ctx, query); err != nil {
		return err
	}

	// Grab shows search suggestions after typing. Tap "See results for ..."
	// to execute the actual search (ENTER just shows suggestions).
	time.Sleep(2 * time.Second)

	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}

	seeResults := finder.First(core.HasText("See results for"))
	if seeResults != nil {
		return seeResults.Tap(ctx, driver.Dev)
	}

	// Fallback: try pressing enter.
	return driver.Dev.KeyEvent(ctx, "KEYCODE_ENTER")
}

// parseRestaurantCards extracts restaurant info from card elements.
// Grab uses different card IDs depending on context: duxton_card on the
// feed, horizontal_merchant_card on search results.
func parseRestaurantCards(finder *core.ElementFinder) []Restaurant {
	var restaurants []Restaurant

	cards := finder.All(func(e *core.Element) bool {
		return e.ResourceID == "com.grabtaxi.passenger:id/duxton_card" ||
			e.ResourceID == "com.grabtaxi.passenger:id/horizontal_merchant_card"
	})
	for _, card := range cards {
		r := parseCard(card)
		if r.Name != "" {
			restaurants = append(restaurants, r)
		}
	}

	return restaurants
}

func parseCard(card *core.Element) Restaurant {
	var r Restaurant
	var textViews []string

	// Collect all text from child TextViews
	collectTexts(card, &textViews)

	// First non-empty text is typically the name, rest are promos/details
	for _, t := range textViews {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if r.Name == "" {
			r.Name = t
		} else if r.Promo == "" && isPromo(t) {
			r.Promo = t
		}
	}

	return r
}

func collectTexts(e *core.Element, texts *[]string) {
	if e.Text != "" {
		*texts = append(*texts, e.Text)
	}
	for _, child := range e.Children {
		collectTexts(child, texts)
	}
}

var promoPattern = regexp.MustCompile(`(?i)(off|free|deal|promo|discount|%|฿)`)

func isPromo(text string) bool {
	return promoPattern.MatchString(text)
}

// ParseRestaurantCardsFromXML is a test helper that parses restaurants from raw XML.
func ParseRestaurantCardsFromXML(xmlData []byte) ([]Restaurant, error) {
	finder, err := core.NewElementFinderFromXML(xmlData)
	if err != nil {
		return nil, err
	}
	return parseRestaurantCards(finder), nil
}
