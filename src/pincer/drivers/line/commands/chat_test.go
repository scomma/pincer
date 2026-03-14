package commands

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

func TestParseChatList(t *testing.T) {
	data := loadFixture(t, "../../../../../tests/fixtures/line/chats.xml")
	chats, err := ParseChatListFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if len(chats) == 0 {
		t.Fatal("expected to find chats")
	}

	// Check first chat
	first := chats[0]
	if first.Name == "" {
		t.Error("expected first chat to have a name")
	}
	t.Logf("First chat: name=%q lastMsg=%q time=%q unread=%d members=%d muted=%v",
		first.Name, first.LastMessage, first.Time, first.UnreadCount, first.MemberCount, first.Muted)

	// Check that we parsed unread counts
	var hasUnread bool
	for _, c := range chats {
		if c.UnreadCount > 0 {
			hasUnread = true
			break
		}
	}
	if !hasUnread {
		t.Error("expected at least one chat with unread messages")
	}

	// Log all chats for inspection
	for i, c := range chats {
		t.Logf("Chat %d: name=%q unread=%d members=%d time=%q", i, c.Name, c.UnreadCount, c.MemberCount, c.Time)
	}
}

func TestParseChatListMemberCount(t *testing.T) {
	data := loadFixture(t, "../../../../../tests/fixtures/line/chats.xml")
	chats, err := ParseChatListFromXML(data)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	// First chat "Project Atlas" should have member count 239
	var found bool
	for _, c := range chats {
		if c.Name == "Project Atlas" {
			found = true
			if c.MemberCount != 239 {
				t.Errorf("expected member count 239, got %d", c.MemberCount)
			}
			if c.UnreadCount != 154 {
				t.Errorf("expected unread count 154, got %d", c.UnreadCount)
			}
			break
		}
	}
	if !found {
		t.Error("expected to find 'Project Atlas' chat")
	}
}
