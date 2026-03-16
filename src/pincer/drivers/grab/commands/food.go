package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/grab"
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
		// Wait for search results to load, and prefer the restaurant-only view
		// when Grab offers a separate "see all restaurants" affordance.
		_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, func(e *core.Element) bool {
			return e.ResourceID == "com.grabtaxi.passenger:id/duxton_see_all_restaurants" ||
				e.ResourceID == "com.grabtaxi.passenger:id/duxton_card" ||
				e.ResourceID == "com.grabtaxi.passenger:id/horizontal_merchant_card"
		})

		finder, err := driver.Workflow.FreshDump(ctx)
		if err == nil {
			if seeAll := finder.ByID("com.grabtaxi.passenger:id/duxton_see_all_restaurants"); seeAll != nil {
				if err := seeAll.Tap(ctx, driver.Dev); err == nil {
					_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, func(e *core.Element) bool {
						return e.ResourceID == "com.grabtaxi.passenger:id/duxton_card" ||
							e.ResourceID == "com.grabtaxi.passenger:id/horizontal_merchant_card"
					})
				}
			}
		}
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
			time.Sleep(250 * time.Millisecond)
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
		time.Sleep(250 * time.Millisecond)
	}
	return nil, core.ErrElementNotFound()
}

func performSearch(ctx context.Context, driver *grab.GrabDriver, query string) error {
	searchBar, err := findSearchBar(ctx, driver)
	if err != nil {
		// Search bar not found — likely on a previous search results
		// screen where the bar is inaccessible. Press back to leave
		// the results, re-navigate to food home, and retry once.
		if err := driver.Dev.KeyEvent(ctx, "KEYCODE_BACK"); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		if err := driver.NavigateToFoodHome(ctx); err != nil {
			return err
		}
		searchBar, err = findSearchBar(ctx, driver)
		if err != nil {
			return err
		}
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
	time.Sleep(500 * time.Millisecond)

	if err := typeVerifiedQuery(ctx, driver, query); err != nil {
		return err
	}

	// Submit immediately before the keyboard/autocomplete mutates the input.
	if err := driver.Dev.KeyEvent(ctx, "KEYCODE_ENTER"); err != nil {
		return err
	}
	if _, err := driver.Workflow.WaitForElement(ctx, 1200*time.Millisecond, func(e *core.Element) bool {
		return e.ResourceID == "com.grabtaxi.passenger:id/duxton_card" ||
			e.ResourceID == "com.grabtaxi.passenger:id/horizontal_merchant_card"
	}); err == nil {
		return nil
	}

	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}

	seeResults := finder.First(core.HasText("See results for"))
	if seeResults != nil {
		return seeResults.Tap(ctx, driver.Dev)
	}

	if exactSuggestion := finder.ByText(query, true); exactSuggestion != nil {
		if tappable := nearestClickable(exactSuggestion); tappable != nil {
			return tappable.Tap(ctx, driver.Dev)
		}
		return exactSuggestion.Tap(ctx, driver.Dev)
	}

	return nil
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

	bestScore := -1
	for _, t := range textViews {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if score := restaurantNameScore(t); score > bestScore {
			bestScore = score
			r.Name = cleanRestaurantName(t)
		}
	}

	for _, t := range textViews {
		t = strings.TrimSpace(t)
		if t == "" || t == r.Name {
			continue
		}
		if r.Promo == "" && isPromo(t) {
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
var deliveryMetaPattern = regexp.MustCompile(`(?i)(mins?|km|delivery|pickup|pick-up|from \d+)`)
var digitsOnlyPattern = regexp.MustCompile(`^\d+$`)

func isPromo(text string) bool {
	return promoPattern.MatchString(text)
}

func restaurantNameScore(text string) int {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)

	switch {
	case trimmed == "":
		return -1
	case lower == "ad":
		return -1
	case lower == "only at grab":
		return -1
	case strings.Contains(lower, "see all restaurants"):
		return -1
	case digitsOnlyPattern.MatchString(trimmed):
		return -1
	case deliveryMetaPattern.MatchString(lower):
		return -1
	case isPromo(trimmed):
		return -1
	}

	score := utf8.RuneCountInString(trimmed)
	if strings.Contains(trimmed, " - ") {
		score += 12
	}
	if strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") {
		score += 4
	}
	if hasLetter(trimmed) {
		score += 8
	}
	if strings.Count(trimmed, " ") >= 1 {
		score += 4
	}

	return score
}

func hasLetter(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func cleanRestaurantName(text string) string {
	return strings.TrimSpace(strings.TrimPrefix(text, "Ad   "))
}

func nearestClickable(el *core.Element) *core.Element {
	for current := el; current != nil; current = current.Parent {
		if current.Clickable {
			return current
		}
	}
	return nil
}

func typeVerifiedQuery(ctx context.Context, driver *grab.GrabDriver, query string) error {
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := driver.Dev.TypeText(ctx, query); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)

		matches, err := queryMatchesInput(ctx, driver, query)
		if err != nil {
			return err
		}
		if matches {
			return nil
		}

		if err := driver.Dev.ClearField(ctx); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}

	return core.NewDriverError("input_mismatch", "search query did not match the text entered in Grab")
}

func queryMatchesInput(ctx context.Context, driver *grab.GrabDriver, query string) (bool, error) {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return false, err
	}

	current := finder.First(func(e *core.Element) bool {
		return e.Class == "android.widget.EditText" && strings.TrimSpace(e.Text) != ""
	})
	if current == nil {
		// No EditText with visible text — the field may not expose typed
		// text via accessibility (common with custom IMEs). Assume success.
		return true, nil
	}

	return strings.EqualFold(strings.TrimSpace(current.Text), strings.TrimSpace(query)), nil
}

// ParseRestaurantCardsFromXML is a test helper that parses restaurants from raw XML.
func ParseRestaurantCardsFromXML(xmlData []byte) ([]Restaurant, error) {
	finder, err := core.NewElementFinderFromXML(xmlData)
	if err != nil {
		return nil, err
	}
	return parseRestaurantCards(finder), nil
}
