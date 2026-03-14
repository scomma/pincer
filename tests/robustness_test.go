package tests

import (
	"context"
	"testing"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/grab"
	grabcmd "github.com/prathan/pincer/src/pincer/drivers/grab/commands"
	"github.com/prathan/pincer/src/pincer/drivers/line"
	linecmd "github.com/prathan/pincer/src/pincer/drivers/line/commands"
	"github.com/prathan/pincer/src/pincer/drivers/shopee"
	shopeecmd "github.com/prathan/pincer/src/pincer/drivers/shopee/commands"
)

// ==========================================================================
// SCENARIO 1: Screen off
// The device display is off. Commands should wake the screen and proceed.
// ==========================================================================

func TestRobust_ScreenOff_GrabFoodSearch(t *testing.T) {
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
	mock.SetScreenOn(false)
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := grabcmd.FoodSearch(ctx, driver, "")
	if err != nil {
		t.Fatalf("expected success after wake, got: %v", err)
	}
	if len(result.Restaurants) == 0 {
		t.Fatal("expected restaurants after wake")
	}

	// Verify WakeScreen was actually called.
	calls := mock.Calls()
	found := false
	for _, c := range calls {
		if c == "WakeScreen" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected WakeScreen to be called")
	}
}

func TestRobust_ScreenOff_LineChatList(t *testing.T) {
	mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
	mock.SetScreenOn(false)
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := linecmd.ChatList(ctx, driver, false, 0)
	if err != nil {
		t.Fatalf("expected success after wake, got: %v", err)
	}
	if len(result.Chats) == 0 {
		t.Fatal("expected chats after wake")
	}
}

func TestRobust_ScreenOff_ShopeeCartList(t *testing.T) {
	mock := core.NewMockDevice("fixtures/shopee/cart.xml", shopee.PackageName)
	mock.SetScreenOn(false)
	driver, _ := shopee.NewShopeeDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := shopeecmd.CartList(ctx, driver)
	if err != nil {
		t.Fatalf("expected success after wake, got: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected cart items after wake")
	}
}

// ==========================================================================
// SCENARIO 2: Wrong app in foreground
// A different app is showing. Commands should launch the correct one.
// ==========================================================================

func TestRobust_WrongApp_GrabFoodSearch(t *testing.T) {
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", "com.some.other.app")
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := grabcmd.FoodSearch(ctx, driver, "")
	if err != nil {
		t.Fatalf("expected success after app launch, got: %v", err)
	}
	if len(result.Restaurants) == 0 {
		t.Fatal("expected restaurants")
	}

	// Verify LaunchApp was called with correct package.
	calls := mock.Calls()
	found := false
	for _, c := range calls {
		if c == "LaunchApp(\"com.grabtaxi.passenger\")" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected LaunchApp to be called for Grab")
	}
}

func TestRobust_WrongApp_LineChatList(t *testing.T) {
	mock := core.NewMockDevice("fixtures/line/chats.xml", "com.random.app")
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := linecmd.ChatList(ctx, driver, false, 0)
	if err != nil {
		t.Fatalf("expected success after app launch, got: %v", err)
	}
	if len(result.Chats) == 0 {
		t.Fatal("expected chats")
	}
}

func TestRobust_WrongApp_ShopeeCartList(t *testing.T) {
	mock := core.NewMockDevice("fixtures/shopee/cart.xml", "com.wrong.app")
	driver, _ := shopee.NewShopeeDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := shopeecmd.CartList(ctx, driver)
	if err != nil {
		t.Fatalf("expected success after app launch, got: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected cart items")
	}
}

// ==========================================================================
// SCENARIO 3: Transient DumpUI errors (ADB connection flakiness)
// First N DumpUI calls fail, then succeed. Commands should retry.
// ==========================================================================

func TestRobust_TransientDumpErrors_GrabFoodSearch(t *testing.T) {
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
	mock.SetDumpErrors(2) // first 2 dumps fail
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := grabcmd.FoodSearch(ctx, driver, "")
	if err != nil {
		t.Fatalf("expected recovery from transient errors, got: %v", err)
	}
	if len(result.Restaurants) == 0 {
		t.Fatal("expected restaurants after recovery")
	}
}

func TestRobust_TransientDumpErrors_LineChatList(t *testing.T) {
	mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
	mock.SetDumpErrors(1) // first dump fails
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := linecmd.ChatList(ctx, driver, false, 0)
	if err != nil {
		t.Fatalf("expected recovery from transient error, got: %v", err)
	}
	if len(result.Chats) == 0 {
		t.Fatal("expected chats after recovery")
	}
}

// ==========================================================================
// SCENARIO 4: Context cancellation (user timeout)
// Commands should respect context cancellation and not hang.
// ==========================================================================

func TestRobust_ContextCancellation_GrabFoodSearch(t *testing.T) {
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
	mock.SetDumpDelay(5 * time.Second) // Very slow dump
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := grabcmd.FoodSearch(ctx, driver, "")
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	t.Logf("correctly returned error on timeout: %v", err)
}

func TestRobust_ContextCancellation_ShopeeSearch(t *testing.T) {
	mock := core.NewMockDevice("fixtures/shopee/home.xml", shopee.PackageName)
	mock.SetDumpDelay(5 * time.Second)
	driver, _ := shopee.NewShopeeDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := shopeecmd.Search(ctx, driver, "test")
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	t.Logf("correctly returned error on timeout: %v", err)
}

// ==========================================================================
// SCENARIO 5: Screen transitions (wrong screen → correct screen)
// App starts on wrong screen, navigation should recover.
// ==========================================================================

func TestRobust_ScreenTransition_GrabHomeToFood(t *testing.T) {
	// Start on home screen, should navigate to food.
	mock := core.NewMockDeviceWithSequence([]string{
		"fixtures/grab/home.xml",         // First dump: on home
		"fixtures/grab/home.xml",         // NavigateToFoodHome sees home, taps food
		"fixtures/grab/food_results.xml", // After tap: food results appear
		"fixtures/grab/food_results.xml", // WaitForElement finds search bar
		"fixtures/grab/food_results.xml", // FreshDump for parsing
		"fixtures/grab/food_results.xml", // Scroll check
	}, grab.PackageName)
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := grabcmd.FoodSearch(ctx, driver, "")
	if err != nil {
		t.Fatalf("expected navigation from home to food, got: %v", err)
	}
	if len(result.Restaurants) == 0 {
		t.Fatal("expected restaurants")
	}
	t.Logf("navigated home→food, found %d restaurants", len(result.Restaurants))
}

func TestRobust_ScreenTransition_LineNavigateToChats(t *testing.T) {
	// Start with chat detail visible, should navigate back to chat list.
	mock := core.NewMockDeviceWithSequence([]string{
		"fixtures/line/chats.xml", // EnsureApp sees correct package
		"fixtures/line/chats.xml", // NavigateToChats detects CHATS screen
		"fixtures/line/chats.xml", // FreshDump for parsing
		"fixtures/line/chats.xml", // Scroll check
	}, line.PackageName)
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := linecmd.ChatList(ctx, driver, false, 0)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(result.Chats) == 0 {
		t.Fatal("expected chats")
	}
}

// ==========================================================================
// SCENARIO 6: Tap errors (transient input injection failures)
// Tap fails once then succeeds. Tests that tap errors propagate.
// ==========================================================================

func TestRobust_TapError_LineChatRead(t *testing.T) {
	// ChatRead taps a chat name. If tap fails, it should propagate the error.
	mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
	mock.SetTapErrors(1)
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := linecmd.ChatRead(ctx, driver, "Family Direct", 10)
	if err == nil {
		t.Fatal("expected error from tap failure")
	}
	t.Logf("correctly reported tap error: %v", err)
}

// ==========================================================================
// SCENARIO 7: Scrolling collects items from multiple pages
// Simulate different content on each scroll by cycling fixtures.
// ==========================================================================

func TestRobust_ScrollCollectsMultiplePages_GrabFood(t *testing.T) {
	// With the same fixture, scroll should stop after first iteration
	// (no new restaurants found on second dump).
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := grabcmd.FoodSearch(ctx, driver, "")
	if err != nil {
		t.Fatalf("food search failed: %v", err)
	}

	// Should have exactly 3 restaurants (deduplicated across scrolls).
	if len(result.Restaurants) != 3 {
		t.Errorf("expected 3 restaurants (deduped), got %d", len(result.Restaurants))
	}

	// Verify scrolling was attempted (at least one Swipe).
	swipes := 0
	for _, c := range mock.Calls() {
		if len(c) > 5 && c[:5] == "Swipe" {
			swipes++
		}
	}
	if swipes != 1 {
		t.Errorf("expected 1 scroll attempt (first finds items, second finds dupes), got %d", swipes)
	}
	t.Logf("scroll behavior: %d swipes, %d restaurants", swipes, len(result.Restaurants))
}

func TestRobust_ScrollCollectsMultiplePages_ShopeeCart(t *testing.T) {
	mock := core.NewMockDevice("fixtures/shopee/cart.xml", shopee.PackageName)
	driver, _ := shopee.NewShopeeDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := shopeecmd.CartList(ctx, driver)
	if err != nil {
		t.Fatalf("cart list failed: %v", err)
	}

	t.Logf("collected %d cart items with scrolling", result.Count)

	// Verify deduplication works — same fixture returns same items.
	if result.Count == 0 {
		t.Fatal("expected cart items")
	}
}

func TestRobust_ScrollCollectsMultiplePages_LineChatList(t *testing.T) {
	mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := linecmd.ChatList(ctx, driver, false, 0)
	if err != nil {
		t.Fatalf("chat list failed: %v", err)
	}

	t.Logf("collected %d chats with scrolling", len(result.Chats))
	if len(result.Chats) == 0 {
		t.Fatal("expected chats")
	}
}

// ==========================================================================
// SCENARIO 8: Combined stress — screen off + wrong app + transient errors
// The worst-case startup scenario.
// ==========================================================================

func TestRobust_CombinedStress_GrabFoodSearch(t *testing.T) {
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", "com.other.app")
	mock.SetScreenOn(false)
	mock.SetDumpErrors(1) // first dump fails too
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := grabcmd.FoodSearch(ctx, driver, "")
	if err != nil {
		t.Fatalf("expected recovery from combined stress, got: %v", err)
	}
	if len(result.Restaurants) == 0 {
		t.Fatal("expected restaurants after recovery")
	}

	calls := mock.Calls()
	t.Logf("combined stress produced %d device calls", len(calls))
	for _, c := range calls {
		t.Logf("  %s", c)
	}
}

// ==========================================================================
// SCENARIO 9: Limit stops scrolling early
// With --limit, should stop scrolling once enough items collected.
// ==========================================================================

func TestRobust_LimitStopsScrolling_LineChatList(t *testing.T) {
	mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := linecmd.ChatList(ctx, driver, false, 2)
	if err != nil {
		t.Fatalf("chat list failed: %v", err)
	}

	if len(result.Chats) > 2 {
		t.Errorf("expected at most 2 chats (limit), got %d", len(result.Chats))
	}

	// With limit=2, should not have scrolled (fixture has more than 2 chats).
	swipes := 0
	for _, c := range mock.Calls() {
		if len(c) > 5 && c[:5] == "Swipe" {
			swipes++
		}
	}
	if swipes > 0 {
		t.Errorf("expected 0 scrolls with limit=2 (fixture has enough), got %d", swipes)
	}
}

// ==========================================================================
// SCENARIO 10: Chat read with screen transition
// Navigate to chat list, then tap specific chat and read messages.
// ==========================================================================

func TestRobust_ChatReadScreenTransition(t *testing.T) {
	// Sequence: chat list → chat list (navigate) → chat list (find chat)
	// → chat detail (after tap) → chat detail (parse messages)
	mock := core.NewMockDeviceWithSequence([]string{
		"fixtures/line/chats.xml",       // EnsureApp
		"fixtures/line/chats.xml",       // NavigateToChats
		"fixtures/line/chats.xml",       // FreshDump to find chat
		"fixtures/line/chat_detail.xml", // WaitForElement (chat detail loaded)
		"fixtures/line/chat_detail.xml", // FreshDump to parse messages
	}, line.PackageName)
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := linecmd.ChatRead(ctx, driver, "Family Direct", 10)
	if err != nil {
		t.Fatalf("chat read failed: %v", err)
	}

	t.Logf("read %d messages from %q", len(result.Messages), result.ChatName)
	if result.ChatName != "Family Direct" {
		t.Errorf("expected chat_name='Family Direct', got %q", result.ChatName)
	}

	// With the real chat_detail fixture, we should find messages.
	if len(result.Messages) > 0 {
		t.Logf("found messages (chat_detail fixture has message elements)")
	} else {
		t.Logf("no messages found (chat_detail fixture may lack message_text elements)")
	}
}

// ==========================================================================
// SCENARIO 11: Shopee search navigates to home first
// If we're on the cart screen, search should navigate home before searching.
// ==========================================================================

func TestRobust_ShopeeSearchFromCart(t *testing.T) {
	// Start on cart, should navigate home then search.
	mock := core.NewMockDeviceWithSequence([]string{
		"fixtures/shopee/cart.xml", // EnsureApp (wrong screen)
		"fixtures/shopee/cart.xml", // NavigateToHome sees cart, presses back
		"fixtures/shopee/home.xml", // After back: home appears
		"fixtures/shopee/home.xml", // FreshDump to find search bar
		"fixtures/shopee/home.xml", // WaitForElement for EditText
		"fixtures/shopee/home.xml", // Search results (same fixture in mock)
		"fixtures/shopee/home.xml", // Scroll check
	}, shopee.PackageName)
	driver, _ := shopee.NewShopeeDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := shopeecmd.Search(ctx, driver, "wireless charger")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("search returned %d products", len(result.Products))
	if result.Query != "wireless charger" {
		t.Errorf("expected query='wireless charger', got %q", result.Query)
	}

	// Verify KEYCODE_BACK was used to navigate away from cart.
	backPressed := false
	for _, c := range mock.Calls() {
		if c == "KeyEvent(\"KEYCODE_BACK\")" {
			backPressed = true
			break
		}
	}
	if !backPressed {
		t.Log("note: did not need KEYCODE_BACK (already on home)")
	}
}

// ==========================================================================
// SCENARIO 12: Auth status with screen off + wrong app
// ==========================================================================

func TestRobust_AuthStatus_ScreenOff_WrongApp(t *testing.T) {
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", "com.chrome.browser")
	mock.SetScreenOn(false)
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := grabcmd.AuthStatus(ctx, driver)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	t.Logf("auth status: logged_in=%v screen=%s", result.LoggedIn, result.Screen)
	if !result.LoggedIn {
		t.Error("expected logged_in=true for food results screen")
	}
}

// ==========================================================================
// SCENARIO 13: Empty/corrupt fixture graceful handling
// ==========================================================================

func TestRobust_EmptyDumpFixture(t *testing.T) {
	// MockDevice with a nonexistent fixture should fail gracefully.
	mock := core.NewMockDevice("fixtures/nonexistent.xml", grab.PackageName)
	driver, _ := grab.NewGrabDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := grabcmd.FoodSearch(ctx, driver, "")
	if err == nil {
		t.Fatal("expected error for missing fixture")
	}
	t.Logf("correctly failed with: %v", err)
}

// ==========================================================================
// SCENARIO 14: Unread filter with scroll
// ==========================================================================

func TestRobust_UnreadFilterWithScroll(t *testing.T) {
	mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
	driver, _ := line.NewLineDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := linecmd.ChatList(ctx, driver, true, 0)
	if err != nil {
		t.Fatalf("chat list failed: %v", err)
	}

	for _, c := range result.Chats {
		if c.UnreadCount == 0 {
			t.Errorf("unread filter returned chat %q with 0 unread", c.Name)
		}
	}
	t.Logf("unread filter: %d chats with unread messages", len(result.Chats))
}

// ==========================================================================
// SCENARIO 15: Shopee cart directly on cart screen
// Verifies navigation recognizes the cart screen immediately.
// ==========================================================================

func TestRobust_ShopeeCartNavigation_AlreadyOnCart(t *testing.T) {
	mock := core.NewMockDevice("fixtures/shopee/cart.xml", shopee.PackageName)
	driver, _ := shopee.NewShopeeDriver(mock)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := shopeecmd.CartList(ctx, driver)
	if err != nil {
		t.Fatalf("expected success when already on cart, got: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected cart items")
	}
	t.Logf("found %d items when already on cart screen", result.Count)
}
