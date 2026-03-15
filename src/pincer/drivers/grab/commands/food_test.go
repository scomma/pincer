package commands

import (
	"os"
	"strings"
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

func TestParseRestaurantCardsSkipsAdLabels(t *testing.T) {
	xml := `<?xml version='1.0' encoding='UTF-8' standalone='yes' ?>
<hierarchy rotation="0">
  <node index="0" text="" resource-id="" class="android.widget.FrameLayout" package="com.grabtaxi.passenger" bounds="[0,0][1080,2400]">
    <node index="0" text="" resource-id="com.grabtaxi.passenger:id/horizontal_merchant_card" class="android.view.ViewGroup" package="com.grabtaxi.passenger" bounds="[0,0][1080,400]">
      <node index="0" text="Only at Grab" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[0,0][200,40]"/>
      <node index="1" text="Ad" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[0,40][100,80]"/>
      <node index="2" text="Saras Veg Food - สุขุมวิท 20" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[0,80][500,120]"/>
      <node index="3" text="20% off" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[0,120][200,160]"/>
    </node>
  </node>
</hierarchy>`

	restaurants, err := ParseRestaurantCardsFromXML([]byte(xml))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if len(restaurants) != 1 {
		t.Fatalf("expected 1 restaurant, got %d", len(restaurants))
	}
	if restaurants[0].Name != "Saras Veg Food - สุขุมวิท 20" {
		t.Fatalf("expected restaurant name to skip ad labels, got %q", restaurants[0].Name)
	}
	if !strings.Contains(restaurants[0].Promo, "20% off") {
		t.Fatalf("expected promo to be preserved, got %q", restaurants[0].Promo)
	}
}
