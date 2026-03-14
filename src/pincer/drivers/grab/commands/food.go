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
		time.Sleep(2 * time.Second) // Wait for results to load
	}

	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	restaurants := parseRestaurantCards(finder)

	return &FoodSearchResult{
		Restaurants: restaurants,
		Query:       query,
	}, nil
}

func performSearch(ctx context.Context, driver *grab.GrabDriver, query string) error {
	// Tap the search bar
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}

	searchBar := finder.ByID("com.grabtaxi.passenger:id/search_bar_clickable_area")
	if searchBar == nil {
		searchBar = finder.ByID("com.grabtaxi.passenger:id/search_entry_content")
	}
	if searchBar == nil {
		return core.ErrElementNotFound()
	}

	c := searchBar.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	// Type the query
	if err := driver.Dev.TypeText(ctx, query); err != nil {
		return err
	}

	// Press Enter to search
	return driver.Dev.KeyEvent(ctx, "KEYCODE_ENTER")
}

// parseRestaurantCards extracts restaurant info from duxton_card elements.
func parseRestaurantCards(finder *core.ElementFinder) []Restaurant {
	var restaurants []Restaurant

	// Find all duxton_card containers (restaurant cards)
	cards := finder.All(core.HasID("com.grabtaxi.passenger:id/duxton_card"))
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
