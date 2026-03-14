package core

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

func TestParseBounds(t *testing.T) {
	tests := []struct {
		input string
		want  Rect
	}{
		{"[0,0][1080,2400]", Rect{0, 0, 1080, 2400}},
		{"[42,1748][415,2383]", Rect{42, 1748, 415, 2383}},
		{"", Rect{}},
	}
	for _, tt := range tests {
		got := parseBounds(tt.input)
		if got != tt.want {
			t.Errorf("parseBounds(%q) = %+v, want %+v", tt.input, got, tt.want)
		}
	}
}

func TestElementCenter(t *testing.T) {
	e := &Element{Bounds: Rect{100, 200, 300, 400}}
	c := e.Center()
	if c.X != 200 || c.Y != 300 {
		t.Errorf("Center() = %+v, want {200, 300}", c)
	}
}

func TestParseGrabHome(t *testing.T) {
	data := loadFixture(t, "../../../tests/fixtures/grab/home.xml")
	finder, err := NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// Should find "Food" text
	food := finder.ByText("Food", true)
	if food == nil {
		t.Fatal("expected to find 'Food' element")
	}

	// Should find bottom nav bar items
	account := finder.ByID("com.grabtaxi.passenger:id/bar_item_account")
	if account == nil {
		t.Fatal("expected to find account bar item")
	}

	// Should find Food content description
	foodDesc := finder.First(HasContentDesc("Food, double tap to select"))
	if foodDesc == nil {
		t.Fatal("expected to find Food content-desc element")
	}
}

func TestParseGrabFoodResults(t *testing.T) {
	data := loadFixture(t, "../../../tests/fixtures/grab/food_results.xml")
	finder, err := NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// Should find duxton_card elements (restaurant cards)
	cards := finder.All(HasID("com.grabtaxi.passenger:id/duxton_card"))
	if len(cards) == 0 {
		t.Fatal("expected to find restaurant cards")
	}
	if len(cards) != 3 {
		t.Errorf("expected 3 restaurant cards, got %d", len(cards))
	}

	// Should find search bar
	searchBar := finder.ByID("com.grabtaxi.passenger:id/search_bar_clickable_area")
	if searchBar == nil {
		t.Fatal("expected to find search bar")
	}
}

func TestByTextCaseInsensitive(t *testing.T) {
	data := loadFixture(t, "../../../tests/fixtures/grab/home.xml")
	finder, err := NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// Case insensitive search
	food := finder.ByText("food", false)
	if food == nil {
		t.Fatal("expected case-insensitive search to find 'food'")
	}
}

func TestByClass(t *testing.T) {
	data := loadFixture(t, "../../../tests/fixtures/grab/home.xml")
	finder, err := NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	textViews := finder.ByClass("android.widget.TextView")
	if len(textViews) == 0 {
		t.Fatal("expected to find TextViews")
	}
}

func TestPredicateCombination(t *testing.T) {
	data := loadFixture(t, "../../../tests/fixtures/grab/home.xml")
	finder, err := NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// Find clickable elements with "Food" in content-desc
	results := finder.All(HasContentDesc("Food"), IsClickable())
	if len(results) == 0 {
		t.Fatal("expected to find clickable Food elements")
	}
}

func TestParseLINEChats(t *testing.T) {
	data := loadFixture(t, "../../../tests/fixtures/line/chats.xml")
	finder, err := NewElementFinderFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// Should find the chat list recycler view
	rv := finder.ByID("jp.naver.line.android:id/chat_list_recycler_view")
	if rv == nil {
		t.Fatal("expected to find chat list recycler view")
	}

	// Should have children (chat items)
	if len(rv.Children) == 0 {
		t.Fatal("expected chat list to have children")
	}

	// First child should have a "name" element
	firstItem := rv.Children[0]
	var foundName bool
	var walkChildren func(*Element)
	walkChildren = func(e *Element) {
		if e.ResourceID == "jp.naver.line.android:id/name" && e.Text != "" {
			foundName = true
		}
		for _, c := range e.Children {
			walkChildren(c)
		}
	}
	walkChildren(firstItem)
	if !foundName {
		t.Fatal("expected first chat item to have a name")
	}
}
