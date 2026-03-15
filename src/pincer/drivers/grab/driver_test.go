package grab

import (
	"context"
	"os"
	"path/filepath"
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

func TestDetectScreenGuestLogin(t *testing.T) {
	const guestLoginXML = `<?xml version='1.0' encoding='UTF-8' standalone='yes' ?>
<hierarchy rotation="0">
  <node index="0" text="" resource-id="" class="android.widget.FrameLayout" package="com.grabtaxi.passenger" bounds="[0,0][1080,2400]">
    <node index="0" text="" resource-id="com.grabtaxi.passenger:id/simple_guest_login_view_title" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[63,1897][1017,1972]"/>
    <node index="1" text="" resource-id="com.grabtaxi.passenger:id/simple_guest_login_view_signup" class="android.view.ViewGroup" package="com.grabtaxi.passenger" clickable="true" bounds="[63,2162][519,2306]">
      <node index="0" text="Sign Up" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[213,2204][370,2264]"/>
    </node>
    <node index="2" text="" resource-id="com.grabtaxi.passenger:id/simple_guest_login_view_login" class="android.view.ViewGroup" package="com.grabtaxi.passenger" clickable="true" bounds="[561,2162][1017,2306]">
      <node index="0" text="Log In" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[727,2204][851,2264]"/>
    </node>
  </node>
</hierarchy>`

	finder, err := core.NewElementFinderFromXML([]byte(guestLoginXML))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if screen := DetectScreen(finder); screen != ScreenLoginGuest {
		t.Fatalf("expected LOGIN_GUEST, got %s", screen)
	}
}

func TestEnsureLoggedInGuestLogin(t *testing.T) {
	const guestLoginXML = `<?xml version='1.0' encoding='UTF-8' standalone='yes' ?>
<hierarchy rotation="0">
  <node index="0" text="" resource-id="" class="android.widget.FrameLayout" package="com.grabtaxi.passenger" bounds="[0,0][1080,2400]">
    <node index="0" text="" resource-id="com.grabtaxi.passenger:id/simple_guest_login_view_title" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[63,1897][1017,1972]"/>
    <node index="1" text="" resource-id="com.grabtaxi.passenger:id/simple_guest_login_view_login" class="android.view.ViewGroup" package="com.grabtaxi.passenger" clickable="true" bounds="[561,2162][1017,2306]">
      <node index="0" text="Log In" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[727,2204][851,2264]"/>
    </node>
  </node>
</hierarchy>`

	dir := t.TempDir()
	path := filepath.Join(dir, "grab_guest_login.xml")
	if err := os.WriteFile(path, []byte(guestLoginXML), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	mock := core.NewMockDevice(path, PackageName)
	driver, err := NewGrabDriver(mock)
	if err != nil {
		t.Fatalf("new driver: %v", err)
	}

	loggedIn, err := driver.EnsureLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("ensure logged in: %v", err)
	}
	if loggedIn {
		t.Fatalf("expected logged out on guest login sheet")
	}
}

func TestDetectScreenGuestStickyFooter(t *testing.T) {
	const guestFooterXML = `<?xml version='1.0' encoding='UTF-8' standalone='yes' ?>
<hierarchy rotation="0">
  <node index="0" text="" resource-id="" class="android.widget.FrameLayout" package="com.grabtaxi.passenger" bounds="[0,0][1080,2400]">
    <node index="0" text="Sign up to do more with Grab!" resource-id="com.grabtaxi.passenger:id/bruce_banner_sub_header" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[42,280][564,333]"/>
    <node index="1" text="" resource-id="com.grabtaxi.passenger:id/newface_guest_browsing_bottom_signup" class="android.view.ViewGroup" package="com.grabtaxi.passenger" clickable="true" bounds="[63,2056][519,2200]">
      <node index="0" text="Sign Up" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[213,2098][370,2158]"/>
    </node>
    <node index="2" text="" resource-id="com.grabtaxi.passenger:id/newface_guest_browsing_bottom_login" class="android.view.ViewGroup" package="com.grabtaxi.passenger" clickable="true" bounds="[582,2056][1038,2200]">
      <node index="0" text="Log In" resource-id="" class="android.widget.TextView" package="com.grabtaxi.passenger" bounds="[748,2098][872,2158]"/>
    </node>
  </node>
</hierarchy>`

	finder, err := core.NewElementFinderFromXML([]byte(guestFooterXML))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if screen := DetectScreen(finder); screen != ScreenLoginGuest {
		t.Fatalf("expected LOGIN_GUEST, got %s", screen)
	}
}
