package grab

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
	// Note: our home.xml fixture was captured while food content was visible,
	// so it has the search bar + restaurant cards. This is the food results screen.
	data := loadFixture(t, "../../../../tests/fixtures/grab/home.xml")
	finder, err := core.NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	screen := DetectScreen(finder)
	// This fixture has search bar + duxton cards, so it's FOOD_RESULTS
	if screen != ScreenFoodResults {
		t.Errorf("expected FOOD_RESULTS, got %s", screen)
	}
}

func TestDetectScreenFoodHome(t *testing.T) {
	data := loadFixture(t, "../../../../tests/fixtures/grab/food_home.xml")
	finder, err := core.NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	screen := DetectScreen(finder)
	if screen != ScreenFoodResults {
		t.Errorf("expected FOOD_RESULTS (has search bar + restaurant cards), got %s", screen)
	}
}

func TestDetectScreenFoodResults(t *testing.T) {
	data := loadFixture(t, "../../../../tests/fixtures/grab/food_results.xml")
	finder, err := core.NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	screen := DetectScreen(finder)
	if screen != ScreenFoodResults {
		t.Errorf("expected FOOD_RESULTS, got %s", screen)
	}
}
