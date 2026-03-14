package commands

import (
	"os"
	"testing"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("loading fixture %s: %v", path, err)
	}
	return data
}

func TestParseRestaurantCards(t *testing.T) {
	data := loadFixture(t, "../../../../../tests/fixtures/grab/food_results.xml")
	restaurants, err := ParseRestaurantCardsFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if len(restaurants) == 0 {
		t.Fatal("expected to find restaurants")
	}

	if len(restaurants) != 3 {
		t.Errorf("expected 3 restaurants, got %d", len(restaurants))
	}

	// Check that we parsed names
	for i, r := range restaurants {
		if r.Name == "" {
			t.Errorf("restaurant %d has empty name", i)
		}
		t.Logf("Restaurant %d: name=%q promo=%q", i, r.Name, r.Promo)
	}

	// First restaurant should have a promo
	if restaurants[0].Promo == "" {
		t.Error("expected first restaurant to have a promo")
	}
}

func TestParseRestaurantCardsFromHome(t *testing.T) {
	data := loadFixture(t, "../../../../../tests/fixtures/grab/food_home.xml")
	restaurants, err := ParseRestaurantCardsFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// food_home should also have restaurant cards
	if len(restaurants) == 0 {
		t.Fatal("expected to find restaurants in food_home")
	}

	for i, r := range restaurants {
		t.Logf("Restaurant %d: name=%q promo=%q", i, r.Name, r.Promo)
	}
}
