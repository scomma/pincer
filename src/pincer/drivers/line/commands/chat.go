package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/line"
)

// ChatEntry represents a single chat in the list.
type ChatEntry struct {
	Name        string `json:"name"`
	LastMessage string `json:"last_message,omitempty"`
	Time        string `json:"time,omitempty"`
	UnreadCount int    `json:"unread_count,omitempty"`
	MemberCount int    `json:"member_count,omitempty"`
	Muted       bool   `json:"muted"`
}

// ChatListResult is the output of `line chat list`.
type ChatListResult struct {
	Chats []ChatEntry `json:"chats"`
}

// ChatList executes the `line chat list` command.
// Scrolls to collect chats beyond the first visible page.
func ChatList(ctx context.Context, driver *line.LineDriver, unreadOnly bool, limit int) (*ChatListResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	// Collect chats across multiple screens by scrolling.
	var chats []ChatEntry
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
		for _, c := range parseChatList(finder) {
			if !seen[c.Name] {
				seen[c.Name] = true
				chats = append(chats, c)
				newCount++
			}
		}

		// Stop scrolling if we already have enough for the limit,
		// or if no new chats appeared (end of list).
		if newCount == 0 || (limit > 0 && len(chats) >= limit) {
			break
		}

		if scroll < maxScrolls {
			if err := driver.Workflow.ScrollDown(ctx); err != nil {
				return nil, err
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	if unreadOnly {
		var filtered []ChatEntry
		for _, c := range chats {
			if c.UnreadCount > 0 {
				filtered = append(filtered, c)
			}
		}
		chats = filtered
	}

	if limit > 0 && len(chats) > limit {
		chats = chats[:limit]
	}

	return &ChatListResult{Chats: chats}, nil
}

func parseChatList(finder *core.ElementFinder) []ChatEntry {
	var chats []ChatEntry

	// Find the recycler view and iterate its children
	rv := finder.ByID("jp.naver.line.android:id/chat_list_recycler_view")
	if rv == nil {
		return nil
	}

	for _, item := range rv.Children {
		chat := parseChatItem(item)
		if chat.Name != "" {
			chats = append(chats, chat)
		}
	}

	return chats
}

func parseChatItem(item *core.Element) ChatEntry {
	var chat ChatEntry

	nameEl := findChildByID(item, "name")
	if nameEl != nil {
		chat.Name = nameEl.Text
	}

	lastMsg := findChildByID(item, "last_message")
	if lastMsg != nil {
		chat.LastMessage = lastMsg.Text
	}

	dateEl := findChildByID(item, "date")
	if dateEl != nil {
		chat.Time = dateEl.Text
	}

	// Check both regular and square chat unread counts
	unreadEl := findChildByID(item, "unread_message_count")
	if unreadEl == nil {
		unreadEl = findChildByID(item, "square_chat_unread_message_count")
	}
	if unreadEl != nil && unreadEl.Text != "" {
		text := strings.ReplaceAll(unreadEl.Text, "+", "")
		if n, err := strconv.Atoi(text); err == nil {
			chat.UnreadCount = n
		}
	}

	memberEl := findChildByID(item, "member_count")
	if memberEl != nil {
		text := strings.Trim(memberEl.Text, "()")
		if n, err := strconv.Atoi(text); err == nil {
			chat.MemberCount = n
		}
	}

	muteEl := findChildByID(item, "notification_off")
	if muteEl != nil {
		// The element exists if the chat is muted (even if it has no text)
		chat.Muted = true
	}

	return chat
}

func findChildByID(e *core.Element, idSuffix string) *core.Element {
	if strings.HasSuffix(e.ResourceID, "/"+idSuffix) {
		return e
	}
	for _, child := range e.Children {
		if found := findChildByID(child, idSuffix); found != nil {
			return found
		}
	}
	return nil
}

// UnreadChat holds one chat's unread messages for ChatReadAllUnread.
type UnreadChat struct {
	Name        string    `json:"name"`
	UnreadCount int       `json:"unread_count"`
	Messages    []Message `json:"messages"`
}

// ChatReadAllUnreadResult is the output of `line chat read-unread`.
type ChatReadAllUnreadResult struct {
	Chats []UnreadChat `json:"chats"`
	Count int          `json:"count"`
}

// ChatReadAllUnread lists unread chats, opens each one, reads messages,
// and returns everything in a single response.
func ChatReadAllUnread(ctx context.Context, driver *line.LineDriver, msgLimit int) (*ChatReadAllUnreadResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	// First pass: collect all unread chat names and counts.
	var unreadChats []ChatEntry
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
		for _, c := range parseChatList(finder) {
			if !seen[c.Name] && c.UnreadCount > 0 {
				seen[c.Name] = true
				unreadChats = append(unreadChats, c)
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
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(unreadChats) == 0 {
		return &ChatReadAllUnreadResult{Count: 0}, nil
	}

	// Second pass: open each unread chat, read messages, go back.
	var results []UnreadChat
	for _, entry := range unreadChats {
		if ctx.Err() != nil {
			break
		}

		// Navigate to chat list first (pressing back if in a chat detail).
		if err := driver.NavigateToChats(ctx); err != nil {
			break
		}

		chatEl, err := scrollToTopAndFindChat(ctx, driver, entry.Name)
		if err != nil {
			// Chat might have scrolled off or name changed. Skip it.
			continue
		}

		if err := chatEl.Tap(ctx, driver.Dev); err != nil {
			continue
		}
		_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, isChatDetail)

		detailFinder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			_ = driver.Dev.KeyEvent(ctx, "KEYCODE_BACK")
			time.Sleep(500 * time.Millisecond)
			continue
		}

		messages := parseChatMessages(detailFinder)
		if msgLimit > 0 && len(messages) > msgLimit {
			messages = messages[len(messages)-msgLimit:]
		}

		results = append(results, UnreadChat{
			Name:        entry.Name,
			UnreadCount: entry.UnreadCount,
			Messages:    messages,
		})

		// Stay briefly so LINE registers the read, then go back.
		time.Sleep(500 * time.Millisecond)
		_ = driver.Dev.KeyEvent(ctx, "KEYCODE_BACK")
		time.Sleep(750 * time.Millisecond)
	}

	return &ChatReadAllUnreadResult{
		Chats: results,
		Count: len(results),
	}, nil
}

// Message represents a single message in a chat.
type Message struct {
	Sender string `json:"sender,omitempty"`
	Text   string `json:"text"`
	Time   string `json:"time,omitempty"`
}

// ChatReadResult is the output of `line chat read`.
type ChatReadResult struct {
	ChatName string    `json:"chat_name"`
	Messages []Message `json:"messages"`
}

// scrollToTopAndFindChat scrolls up to the top of the chat list, then
// scrolls down looking for a chat by exact name. Returns the element if
// found, or an error.
func scrollToTopAndFindChat(ctx context.Context, driver *line.LineDriver, chatName string) (*core.Element, error) {
	// Scroll to the top first (previous commands may have scrolled down).
	for i := 0; i < 3; i++ {
		if err := driver.Workflow.ScrollUp(ctx); err != nil {
			return nil, err
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Scroll down looking for the target chat.
	const maxScrolls = 10
	for scroll := 0; scroll <= maxScrolls; scroll++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		finder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		if el := finder.ByText(chatName, true); el != nil {
			return el, nil
		}

		if scroll < maxScrolls {
			if err := driver.Workflow.ScrollDown(ctx); err != nil {
				return nil, err
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil, core.NewDriverError("chat_not_found", "Chat '"+chatName+"' not found after scrolling")
}

// ChatRead executes the `line chat read` command.
func ChatRead(ctx context.Context, driver *line.LineDriver, chatName string, limit int) (*ChatReadResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	chatEl, err := scrollToTopAndFindChat(ctx, driver, chatName)
	if err != nil {
		return nil, err
	}

	if err := chatEl.Tap(ctx, driver.Dev); err != nil {
		return nil, err
	}

	// Wait for chat detail screen to load.
	_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, isChatDetail)

	detailFinder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	messages := parseChatMessages(detailFinder)
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	return &ChatReadResult{
		ChatName: chatName,
		Messages: messages,
	}, nil
}

// isChatDetail matches elements that indicate the LINE chat detail screen.
var isChatDetail core.Predicate = func(e *core.Element) bool {
	for _, id := range line.ChatDetailIDs {
		if e.ResourceID == id {
			return true
		}
	}
	return false
}

func parseChatMessages(finder *core.ElementFinder) []Message {
	var messages []Message

	// LINE uses several resource IDs for message content across versions:
	//   chat_ui_message_text, message_text, chathistory_message
	msgElements := finder.All(func(e *core.Element) bool {
		return strings.Contains(e.ResourceID, "message_text") ||
			strings.Contains(e.ResourceID, "chathistory_message")
	})

	for _, el := range msgElements {
		if el.Text != "" {
			messages = append(messages, Message{Text: el.Text})
		}
	}

	return messages
}

// ChatSendResult is the output of `line chat send`.
type ChatSendResult struct {
	ChatName string `json:"chat_name"`
	Message  string `json:"message,omitempty"`
	Location string `json:"location,omitempty"`
}

const (
	sendButtonID    = "jp.naver.line.android:id/chat_ui_send_button_image"
	messageBoxID    = "jp.naver.line.android:id/chat_ui_message_edit"
	keyboardBtnID   = "jp.naver.line.android:id/chat_ui_oa_bottombar_keyboard_button"
	confirmButtonID = "jp.naver.line.android:id/confirm_button"
	attachButtonID  = "jp.naver.line.android:id/chathistory_attach_button"
	locSearchID     = "jp.naver.line.android:id/location_search_text"
	locTitleID      = "jp.naver.line.android:id/title"
	locShareBtnID   = "jp.naver.line.android:id/header_button_text"
)

// ChatSend sends a message to a LINE chat.
func ChatSend(ctx context.Context, driver *line.LineDriver, chatName string, message string) (*ChatSendResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	chatEl, err := scrollToTopAndFindChat(ctx, driver, chatName)
	if err != nil {
		return nil, err
	}

	if err := chatEl.Tap(ctx, driver.Dev); err != nil {
		return nil, err
	}

	// Wait for the chat detail screen to load.
	_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, isChatDetail)

	// Dismiss any info popup (e.g. Keep Memo's first-visit dialog).
	if err := dismissLinePopup(ctx, driver); err != nil {
		return nil, err
	}

	// Find the message input field.
	msgBox, err := ensureMessageInput(ctx, driver)
	if err != nil {
		return nil, err
	}
	c := msgBox.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}
	time.Sleep(300 * time.Millisecond)

	// Type the message.
	if err := driver.Dev.TypeText(ctx, message); err != nil {
		return nil, fmt.Errorf("typing message: %w", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Wait for the send button to become active (content-desc changes
	// from "Voice message" to "Send" when text is present).
	sendBtn, err := driver.Workflow.WaitForElement(ctx, 3*time.Second, func(e *core.Element) bool {
		return e.ResourceID == sendButtonID && e.ContentDesc == "Send"
	})
	if err != nil {
		return nil, core.NewDriverError("send_not_ready", "send button did not activate after typing")
	}

	c = sendBtn.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, fmt.Errorf("tapping send: %w", err)
	}

	// Wait for the input field to clear, confirming the message was sent.
	_, _ = driver.Workflow.WaitForElement(ctx, 3*time.Second, func(e *core.Element) bool {
		return e.ResourceID == sendButtonID && e.ContentDesc == "Voice message"
	})

	return &ChatSendResult{
		ChatName: chatName,
		Message:  message,
	}, nil
}

// dismissLinePopup taps any visible "OK" / "confirm_button" overlay.
// These appear on first visits (e.g. Keep Memo's info dialog).
func dismissLinePopup(ctx context.Context, driver *line.LineDriver) error {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}
	btn := finder.ByID(confirmButtonID)
	if btn == nil {
		return nil // No popup — nothing to dismiss.
	}
	c := btn.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

// ensureMessageInput returns the message input field. If it's not visible
// (e.g. Official Account chats show a rich menu instead), taps the
// keyboard toggle to reveal it.
func ensureMessageInput(ctx context.Context, driver *line.LineDriver) (*core.Element, error) {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	if msgBox := finder.ByID(messageBoxID); msgBox != nil {
		return msgBox, nil
	}

	// Official Account chats have a keyboard toggle button.
	kbBtn := finder.ByID(keyboardBtnID)
	if kbBtn == nil {
		return nil, core.NewDriverError("element_not_found", "message input field not found")
	}
	c := kbBtn.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}

	msgBox, err := driver.Workflow.WaitForElement(ctx, 3*time.Second, core.HasID(messageBoxID))
	if err != nil {
		return nil, core.NewDriverError("element_not_found", "message input did not appear after keyboard toggle")
	}
	return msgBox, nil
}

// ChatSendLocation shares a location in a LINE chat.
// If query is "current", shares the device's current GPS location.
// Otherwise, searches for the query and picks the best match.
func ChatSendLocation(ctx context.Context, driver *line.LineDriver, chatName string, query string) (*ChatSendResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	chatEl, err := scrollToTopAndFindChat(ctx, driver, chatName)
	if err != nil {
		return nil, err
	}

	if err := chatEl.Tap(ctx, driver.Dev); err != nil {
		return nil, err
	}
	_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, isChatDetail)

	if err := dismissLinePopup(ctx, driver); err != nil {
		return nil, err
	}

	// Open the attachment menu and tap Location.
	if err := openLocationPicker(ctx, driver); err != nil {
		return nil, err
	}

	// Dismiss any permission dialog that appears on first use.
	if err := dismissPermissionDialog(ctx, driver); err != nil {
		return nil, err
	}

	// Wait for the location picker to finish loading.
	_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second,
		core.HasID(locSearchID))

	if strings.EqualFold(query, "current") {
		return shareCurrentLocation(ctx, driver, chatName)
	}
	return searchAndShareLocation(ctx, driver, chatName, query)
}

func openLocationPicker(ctx context.Context, driver *line.LineDriver) error {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}

	attachBtn := finder.ByID(attachButtonID)
	if attachBtn == nil {
		return core.NewDriverError("element_not_found", "attachment button not found")
	}
	c := attachBtn.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)

	// Find and tap the "Location" item in the attachment grid.
	finder, err = driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}
	// Find the "Location" item specifically inside the attachment grid,
	// NOT a location message in the chat (which also has "Location" text).
	locItem := finder.First(func(e *core.Element) bool {
		return e.ResourceID == "jp.naver.line.android:id/chat_ui_attach_item_text" &&
			strings.EqualFold(e.Text, "Location")
	})
	if locItem == nil {
		return core.NewDriverError("element_not_found", "Location option not found in attachment menu")
	}

	// The text itself isn't clickable — find its clickable parent.
	clickable := locItem
	for p := locItem.Parent; p != nil; p = p.Parent {
		if p.Clickable {
			clickable = p
			break
		}
	}
	c = clickable.Center()
	return driver.Dev.Tap(ctx, c.X, c.Y)
}

func dismissPermissionDialog(ctx context.Context, driver *line.LineDriver) error {
	time.Sleep(1 * time.Second)
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}
	// Android permission dialog — tap "While using the app" or "Allow".
	allow := finder.First(func(e *core.Element) bool {
		return strings.Contains(e.ResourceID, "permission_allow_foreground_only_button") ||
			strings.Contains(e.ResourceID, "permission_allow_one_time_button")
	})
	if allow == nil {
		// Also check for generic "Allow" text.
		allow = finder.ByText("While using the app", false)
	}
	if allow == nil {
		return nil // No permission dialog — already granted.
	}
	c := allow.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	return nil
}

func shareCurrentLocation(ctx context.Context, driver *line.LineDriver, chatName string) (*ChatSendResult, error) {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}
	shareBtn := finder.First(func(e *core.Element) bool {
		return e.ResourceID == locShareBtnID && strings.EqualFold(e.Text, "Share")
	})
	if shareBtn == nil {
		return nil, core.NewDriverError("element_not_found", "Share button not found on location picker")
	}
	c := shareBtn.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}
	// LINE shows a location viewer after sharing. Dismiss it.
	if err := dismissLocationViewer(ctx, driver); err != nil {
		return nil, err
	}

	return &ChatSendResult{
		ChatName: chatName,
		Location: "current",
	}, nil
}

func searchAndShareLocation(ctx context.Context, driver *line.LineDriver, chatName, query string) (*ChatSendResult, error) {
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	searchBox := finder.ByID(locSearchID)
	if searchBox == nil {
		return nil, core.NewDriverError("element_not_found", "location search box not found")
	}
	c := searchBox.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}
	time.Sleep(300 * time.Millisecond)

	if err := driver.Dev.TypeText(ctx, query); err != nil {
		return nil, fmt.Errorf("typing location query: %w", err)
	}
	time.Sleep(2 * time.Second) // Wait for search results

	// Collect results across multiple pages of results.
	var allTitles []*core.Element
	for page := 0; page < 4; page++ {
		finder, err = driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}
		allTitles = append(allTitles, finder.All(core.HasID(locTitleID))...)
		if page < 3 {
			_ = driver.Workflow.ScrollDown(ctx)
			time.Sleep(500 * time.Millisecond)
		}
	}

	best := pickBestFromElements(allTitles, query)
	if best == nil {
		return nil, core.NewDriverError("location_not_found",
			"no suitable location found for '"+query+"'")
	}

	// Tap the best result — find its clickable ancestor.
	clickable := best
	for p := best.Parent; p != nil; p = p.Parent {
		if p.Clickable {
			clickable = p
			break
		}
	}
	c = clickable.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}
	time.Sleep(1 * time.Second)

	// The map centers on the selection. Now tap Share.
	finder, err = driver.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}
	shareBtn := finder.First(func(e *core.Element) bool {
		return e.ResourceID == locShareBtnID && strings.EqualFold(e.Text, "Share")
	})
	if shareBtn == nil {
		return nil, core.NewDriverError("element_not_found", "Share button not found after selecting location")
	}
	c = shareBtn.Center()
	if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}
	if err := dismissLocationViewer(ctx, driver); err != nil {
		return nil, err
	}

	return &ChatSendResult{
		ChatName: chatName,
		Location: strings.TrimSpace(best.Text),
	}, nil
}

// dismissLocationViewer presses back to dismiss the full-screen
// location viewer that LINE shows after sharing a location.
func dismissLocationViewer(ctx context.Context, driver *line.LineDriver) error {
	time.Sleep(2 * time.Second)
	// Check if we're on the location viewer (has location_viewer_menu).
	finder, err := driver.Workflow.FreshDump(ctx)
	if err != nil {
		return err
	}
	if finder.ByID("jp.naver.line.android:id/location_viewer_menu") != nil {
		if err := driver.Dev.KeyEvent(ctx, "KEYCODE_BACK"); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	return nil
}

// pickBestFromElements selects the most relevant location from a list
// of title elements. Prefers results where the title closely matches
// the query without extra venue specifics.
func pickBestFromElements(titles []*core.Element, query string) *core.Element {
	queryLower := strings.ToLower(query)

	var bestEl *core.Element
	bestScore := -1

	for _, el := range titles {
		text := strings.TrimSpace(el.Text)
		if text == "" {
			continue
		}
		score := locationMatchScore(text, queryLower)
		if score > bestScore {
			bestScore = score
			bestEl = el
		}
	}
	return bestEl
}

// locationMatchScore scores how well a location title matches the query.
// Higher scores are better. Returns -1 for non-matches.
func locationMatchScore(title, queryLower string) int {
	titleLower := strings.ToLower(title)

	if !strings.Contains(titleLower, queryLower) {
		return -1
	}

	score := 100

	// Prefer titles that START with the query (main venue, not a sub-venue).
	if strings.HasPrefix(titleLower, queryLower) {
		score += 80
	}

	// Heavy penalty for "@" which means "sub-venue at CentralWorld".
	if strings.Contains(titleLower, "@") {
		score -= 40
	}

	// Penalize titles with floor/zone/venue indicators (sub-venues).
	for _, noise := range []string{"fl.", "floor", "zone", "office", "shop", "cafe", "meeting", "live", "samsung", "brand"} {
		if strings.Contains(titleLower, noise) {
			score -= 15
		}
	}

	// Boost landmark/plaza terms — these tend to be the main venue entry.
	for _, landmark := range []string{"square", "plaza", "complex", "center", "centre"} {
		if strings.Contains(titleLower, landmark) {
			score += 30
		}
	}

	// Prefer shorter titles (less likely to be a compound sub-venue name).
	score -= len(title) / 2

	return score
}

// ParseChatListFromXML is a test helper for parsing chats from raw XML.
func ParseChatListFromXML(xmlData []byte) ([]ChatEntry, error) {
	finder, err := core.NewElementFinderFromXML(xmlData)
	if err != nil {
		return nil, err
	}
	return parseChatList(finder), nil
}
