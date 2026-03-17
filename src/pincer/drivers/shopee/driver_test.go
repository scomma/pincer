package shopee

import (
	"os"
	"testing"

	"github.com/prathan/pincer/src/pincer/core"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("loading fixture %s: %v", path, err)
	}
	return data
}

func TestDetectScreenHome(t *testing.T) {
	data := loadFixture(t, "../../../../tests/fixtures/shopee/home.xml")
	finder, err := core.NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	screen := DetectScreen(finder)
	if screen != ScreenHome {
		t.Errorf("expected HOME, got %s", screen)
	}
}

func TestDetectScreenCart(t *testing.T) {
	data := loadFixture(t, "../../../../tests/fixtures/shopee/cart.xml")
	finder, err := core.NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	screen := DetectScreen(finder)
	if screen != ScreenCart {
		t.Errorf("expected CART, got %s", screen)
	}
}

func TestDetectScreenOrders(t *testing.T) {
	data := loadFixture(t, "../../../../tests/fixtures/shopee/orders.xml")
	finder, err := core.NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	screen := DetectScreen(finder)
	if screen != ScreenOrders {
		t.Errorf("expected ORDERS, got %s", screen)
	}
}
