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

func TestParseCartItems(t *testing.T) {
	data := loadFixture(t, "../../../../../tests/fixtures/shopee/cart.xml")
	items, err := ParseCartItemsFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("expected to find cart items")
	}

	for i, item := range items {
		t.Logf("Item %d: shop=%q name=%q variation=%q price=%q oldPrice=%q qty=%q",
			i, item.Shop, item.Name, item.Variation, item.Price, item.OldPrice, item.Quantity)
	}

	// Should find the Northstar item
	var foundEnergea bool
	for _, item := range items {
		if item.Shop == "Northstar Flagship Store" {
			foundEnergea = true
			if item.Variation != "GRAPHITE" {
				t.Errorf("expected GRAPHITE variation, got %q", item.Variation)
			}
			break
		}
	}
	if !foundEnergea {
		t.Error("expected to find Northstar Flagship Store item")
	}

	// Should find Orbit Labs items
	var inovagenCount int
	for _, item := range items {
		if item.Shop == "Orbit Labs TH" {
			inovagenCount++
		}
	}
	if inovagenCount != 2 {
		t.Errorf("expected 2 Orbit Labs TH items, got %d", inovagenCount)
	}
}
