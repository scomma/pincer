package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prathan/pincer/src/pincer/bridges/line"
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
func ChatList(ctx context.Context, bridge *line.LineBridge, unreadOnly bool, limit int) (*ChatListResult, error) {
	if err := bridge.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := bridge.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	finder, err := bridge.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	chats := parseChatList(finder)

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
func ChatRead(ctx context.Context, bridge *line.LineBridge, chatName string, limit int) (*ChatReadResult, error) {
	if err := bridge.EnsureAppRunning(ctx); err != nil {
		return nil, fmt.Errorf("ensure app running: %w", err)
	}

	if err := bridge.NavigateToChats(ctx); err != nil {
		return nil, fmt.Errorf("navigate to chats: %w", err)
	}

	// Find and tap the chat
	finder, err := bridge.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	chatEl := finder.ByText(chatName, true)
	if chatEl == nil {
		return nil, core.NewBridgeError("chat_not_found", "Chat '"+chatName+"' not found in visible list")
	}

	c := chatEl.Center()
	if err := bridge.Dev.Tap(ctx, c.X, c.Y); err != nil {
		return nil, err
	}

	// Wait for chat detail to load
	time.Sleep(2 * time.Second)

	finder, err = bridge.Workflow.FreshDump(ctx)
	if err != nil {
		return nil, err
	}

	messages := parseChatMessages(finder)
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

	// Look for message text elements. LINE messages typically use
	// specific resource IDs for message content.
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
