package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/drivers/line"
	"github.com/prathan/pincer/src/pincer/core"
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
			if err := driver.Dev.Swipe(ctx, 540, 1600, 540, 800, 300); err != nil {
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

// ChatRead executes the `line chat read` command.
func ChatRead(ctx context.Context, driver *line.LineDriver, chatName string, limit int) (*ChatReadResult, error) {
	if err := driver.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := driver.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	// Scroll up to the top of the chat list first (previous commands may
	// have scrolled it down), then scroll down looking for the target chat.
	for i := 0; i < 3; i++ {
		_ = driver.Dev.Swipe(ctx, 540, 400, 540, 1600, 200)
		time.Sleep(300 * time.Millisecond)
	}

	// Look for the target chat, scrolling down if needed.
	const maxScrolls = 10
	for scroll := 0; scroll <= maxScrolls; scroll++ {
		finder, err := driver.Workflow.FreshDump(ctx)
		if err != nil {
			return nil, err
		}

		chatEl := finder.ByText(chatName, true)
		if chatEl != nil {
			c := chatEl.Center()
			if err := driver.Dev.Tap(ctx, c.X, c.Y); err != nil {
				return nil, err
			}
			goto chatOpened
		}

		if scroll < maxScrolls {
			if err := driver.Dev.Swipe(ctx, 540, 1600, 540, 800, 300); err != nil {
				return nil, err
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil, core.NewDriverError("chat_not_found", "Chat '"+chatName+"' not found after scrolling")

chatOpened:
	// Wait for chat detail screen to load.
	_, _ = driver.Workflow.WaitForElement(ctx, 5*time.Second, func(e *core.Element) bool {
		return e.ResourceID == "jp.naver.line.android:id/chathistory_message_edit_text" ||
			e.ResourceID == "jp.naver.line.android:id/chat_ui_message_edit" ||
			e.ResourceID == "jp.naver.line.android:id/chathistory_message_list"
	})

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

// ParseChatListFromXML is a test helper for parsing chats from raw XML.
func ParseChatListFromXML(xmlData []byte) ([]ChatEntry, error) {
	finder, err := core.NewElementFinderFromXML(xmlData)
	if err != nil {
		return nil, err
	}
	return parseChatList(finder), nil
}
