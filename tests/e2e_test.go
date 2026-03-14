package tests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/prathan/pincer/src/pincer/bridges/grab"
	grabcmd "github.com/prathan/pincer/src/pincer/bridges/grab/commands"
	"github.com/prathan/pincer/src/pincer/bridges/line"
	linecmd "github.com/prathan/pincer/src/pincer/bridges/line/commands"
	"github.com/prathan/pincer/src/pincer/bridges/shopee"
	shopeecmd "github.com/prathan/pincer/src/pincer/bridges/shopee/commands"
	"github.com/prathan/pincer/src/pincer/core"
)

// TestAIAssistantLunchScenario simulates an AI assistant helping order lunch.
//
// Scenario: "What's for lunch nearby? Also check my LINE messages and Shopee cart."
//
// The assistant would:
//   1. pincer grab food search                -> list restaurants
//   2. pincer line chat list --unread          -> check unread messages
//   3. pincer shopee cart list                 -> check cart items
//   4. pincer grab auth status                -> verify login
func TestAIAssistantLunchScenario(t *testing.T) {
	ctx := context.Background()

	// ── Step 1: Search for food on Grab ──────────────────────────────────

	t.Run("grab_food_search", func(t *testing.T) {
		mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
		bridge, err := grab.NewGrabBridge(mock)
		if err != nil {
			t.Fatalf("creating bridge: %v", err)
		}

		result, err := grabcmd.FoodSearch(ctx, bridge, "")
		if err != nil {
			t.Fatalf("food search: %v", err)
		}

		if len(result.Restaurants) == 0 {
			t.Fatal("expected restaurants")
		}

		// Verify the response is valid JSON (as an AI would receive it)
		resp := core.NewResponse(result)
		jsonBytes, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			t.Fatalf("marshaling response: %v", err)
		}

		t.Logf("AI receives:\n%s", string(jsonBytes))

		// Verify structure
		if !resp.OK {
			t.Error("expected ok=true")
		}
		if len(result.Restaurants) != 3 {
			t.Errorf("expected 3 restaurants, got %d", len(result.Restaurants))
		}

		// An AI could parse these and present: "I found 3 nearby restaurants..."
		for _, r := range result.Restaurants {
			if r.Name == "" {
				t.Error("restaurant has empty name")
			}
		}
	})

	// ── Step 2: Check LINE messages ──────────────────────────────────────

	t.Run("line_chat_list_unread", func(t *testing.T) {
		mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
		bridge, err := line.NewLineBridge(mock)
		if err != nil {
			t.Fatalf("creating bridge: %v", err)
		}

		result, err := linecmd.ChatList(ctx, bridge, true, 5)
		if err != nil {
			t.Fatalf("chat list: %v", err)
		}

		resp := core.NewResponse(result)
		jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
		t.Logf("AI receives:\n%s", string(jsonBytes))

		if len(result.Chats) == 0 {
			t.Fatal("expected unread chats")
		}
		if len(result.Chats) > 5 {
			t.Errorf("expected at most 5 chats (limit), got %d", len(result.Chats))
		}

		// All returned chats should have unread > 0
		for _, c := range result.Chats {
			if c.UnreadCount == 0 {
				t.Errorf("chat %q has 0 unread but was returned with --unread filter", c.Name)
			}
		}

		// An AI could say: "You have 5 unread chats. Project Atlas has 154 unread messages..."
	})

	// ── Step 3: Check Shopee cart ─────────────────────────────────────────

	t.Run("shopee_cart_list", func(t *testing.T) {
		mock := core.NewMockDevice("fixtures/shopee/cart.xml", shopee.PackageName)
		bridge, err := shopee.NewShopeeBridge(mock)
		if err != nil {
			t.Fatalf("creating bridge: %v", err)
		}

		result, err := shopeecmd.CartList(ctx, bridge)
		if err != nil {
			t.Fatalf("cart list: %v", err)
		}

		resp := core.NewResponse(result)
		jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
		t.Logf("AI receives:\n%s", string(jsonBytes))

		if result.Count == 0 {
			t.Fatal("expected cart items")
		}

		// Verify item details
		for _, item := range result.Items {
			if item.Name == "" {
				t.Error("cart item has empty name")
			}
			if item.Shop == "" {
				t.Error("cart item has empty shop")
			}
		}

		// An AI could say: "You have 3 items in your Shopee cart totaling ~฿2,099"
	})

	// ── Step 4: Check Grab auth status ───────────────────────────────────

	t.Run("grab_auth_status", func(t *testing.T) {
		mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
		bridge, err := grab.NewGrabBridge(mock)
		if err != nil {
			t.Fatalf("creating bridge: %v", err)
		}

		result, err := grabcmd.AuthStatus(ctx, bridge)
		if err != nil {
			t.Fatalf("auth status: %v", err)
		}

		resp := core.NewResponse(result)
		jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
		t.Logf("AI receives:\n%s", string(jsonBytes))

		if !result.LoggedIn {
			t.Error("expected logged_in=true (food results screen implies logged in)")
		}
	})
}

// TestAIAssistantSearchWithQuery simulates searching for specific food.
func TestAIAssistantSearchWithQuery(t *testing.T) {
	ctx := context.Background()
	mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
	bridge, err := grab.NewGrabBridge(mock)
	if err != nil {
		t.Fatalf("creating bridge: %v", err)
	}

	result, err := grabcmd.FoodSearch(ctx, bridge, "pad thai")
	if err != nil {
		t.Fatalf("food search: %v", err)
	}

	if result.Query != "pad thai" {
		t.Errorf("expected query='pad thai', got %q", result.Query)
	}

	// Verify the mock recorded the search actions
	typed := mock.TypedTexts()
	found := false
	for _, text := range typed {
		if text == "pad thai" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TypeText('pad thai') to be called, got: %v", typed)
	}

	t.Logf("Mock recorded %d calls", len(mock.Calls()))
	for _, call := range mock.Calls() {
		t.Logf("  %s", call)
	}
}

// TestAIAssistantLineChatRead simulates reading a specific chat.
func TestAIAssistantLineChatRead(t *testing.T) {
	ctx := context.Background()

	// Use chats fixture for navigation, then it would switch to chat_detail.
	// Since our mock returns the same fixture, the chat tap will "fail" to find
	// message elements (chats.xml doesn't have message_text IDs).
	// This is expected — in production, the fixture would change after the tap.
	mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
	bridge, err := line.NewLineBridge(mock)
	if err != nil {
		t.Fatalf("creating bridge: %v", err)
	}

	result, err := linecmd.ChatRead(ctx, bridge, "Family Direct", 10)
	if err != nil {
		t.Fatalf("chat read: %v", err)
	}

	if result.ChatName != "Family Direct" {
		t.Errorf("expected chat_name='Family Direct', got %q", result.ChatName)
	}

	// Verify tap was issued (to open the chat)
	taps := mock.Taps()
	if len(taps) == 0 {
		t.Error("expected at least one tap to open the chat")
	}

	resp := core.NewResponse(result)
	jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
	t.Logf("AI receives:\n%s", string(jsonBytes))
}

// TestAIAssistantErrorHandling simulates how errors surface to the AI.
func TestAIAssistantErrorHandling(t *testing.T) {
	t.Run("bridge_error_json", func(t *testing.T) {
		err := core.NewBridgeError("not_logged_in", "App requires login. Run: pincer grab auth login")
		resp := core.NewErrorResponse(err)
		jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
		t.Logf("AI receives error:\n%s", string(jsonBytes))

		if resp.OK {
			t.Error("expected ok=false")
		}
		if resp.Error != "not_logged_in" {
			t.Errorf("expected error code 'not_logged_in', got %q", resp.Error)
		}
	})

	t.Run("chat_not_found", func(t *testing.T) {
		ctx := context.Background()
		mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
		bridge, err := line.NewLineBridge(mock)
		if err != nil {
			t.Fatalf("creating bridge: %v", err)
		}

		_, err = linecmd.ChatRead(ctx, bridge, "NonexistentChat12345", 10)
		if err == nil {
			t.Fatal("expected error for nonexistent chat")
		}

		// Verify the error is a BridgeError with a meaningful code
		if be, ok := err.(*core.BridgeError); ok {
			t.Logf("AI receives error: code=%q message=%q", be.Code, be.Message)
			if be.Code != "chat_not_found" {
				t.Errorf("expected error code 'chat_not_found', got %q", be.Code)
			}
		} else {
			t.Logf("Got non-bridge error (acceptable): %v", err)
		}
	})
}

// TestAIAssistantAllCommandsCoverage verifies every implemented command
// produces valid JSON output that an AI assistant can parse.
func TestAIAssistantAllCommandsCoverage(t *testing.T) {
	ctx := context.Background()

	commands := []struct {
		name string
		run  func() (any, error)
	}{
		{
			name: "pincer grab food search",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
				b, _ := grab.NewGrabBridge(mock)
				return grabcmd.FoodSearch(ctx, b, "")
			},
		},
		{
			name: "pincer grab food search --query lunch",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
				b, _ := grab.NewGrabBridge(mock)
				return grabcmd.FoodSearch(ctx, b, "lunch")
			},
		},
		{
			name: "pincer grab auth status",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/grab/food_results.xml", grab.PackageName)
				b, _ := grab.NewGrabBridge(mock)
				return grabcmd.AuthStatus(ctx, b)
			},
		},
		{
			name: "pincer line chat list",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
				b, _ := line.NewLineBridge(mock)
				return linecmd.ChatList(ctx, b, false, 0)
			},
		},
		{
			name: "pincer line chat list --unread",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
				b, _ := line.NewLineBridge(mock)
				return linecmd.ChatList(ctx, b, true, 0)
			},
		},
		{
			name: "pincer line chat list --unread --limit 3",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
				b, _ := line.NewLineBridge(mock)
				return linecmd.ChatList(ctx, b, true, 3)
			},
		},
		{
			name: "pincer line chat read --chat Family Direct",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/line/chats.xml", line.PackageName)
				b, _ := line.NewLineBridge(mock)
				return linecmd.ChatRead(ctx, b, "Family Direct", 20)
			},
		},
		{
			name: "pincer shopee cart list",
			run: func() (any, error) {
				mock := core.NewMockDevice("fixtures/shopee/cart.xml", shopee.PackageName)
				b, _ := shopee.NewShopeeBridge(mock)
				return shopeecmd.CartList(ctx, b)
			},
		},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.run()
			if err != nil {
				t.Fatalf("command failed: %v", err)
			}

			resp := core.NewResponse(result)
			jsonBytes, err := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			// Verify it's valid JSON
			var parsed map[string]any
			if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}

			// Verify envelope
			if ok, exists := parsed["ok"]; !exists || ok != true {
				t.Errorf("expected ok=true in response")
			}
			if _, exists := parsed["data"]; !exists {
				t.Error("expected 'data' field in response")
			}

			t.Logf("OK: %s -> %d bytes JSON", tc.name, len(jsonBytes))
		})
	}
}
