package line

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
)

const (
	PackageName = "jp.naver.line.android"
	AppTimeout  = 10 * time.Second
)

type Screen string

const (
	ScreenChats      Screen = "CHATS"
	ScreenChatDetail Screen = "CHAT_DETAIL"
	ScreenFriends    Screen = "FRIENDS"
	ScreenUnknown    Screen = "UNKNOWN"
)

// LineBridge implements the Bridge interface for the LINE app.
type LineBridge struct {
	Dev      core.Device
	Workflow *core.Workflow
	Cache    *core.Cache
}

func NewLineBridge(dev core.Device) (*LineBridge, error) {
	cache, err := core.NewCache("line")
	if err != nil {
		return nil, err
	}
	return &LineBridge{
		Dev:      dev,
		Workflow: core.NewWorkflow(dev),
		Cache:    cache,
	}, nil
}

func (b *LineBridge) PackageName() string {
	return PackageName
}

func (b *LineBridge) EnsureAppRunning(ctx context.Context) error {
	return b.Workflow.EnsureApp(ctx, PackageName, AppTimeout)
}

func (b *LineBridge) EnsureLoggedIn(ctx context.Context) (bool, error) {
	finder, err := b.Workflow.FreshDump(ctx)
	if err != nil {
		return false, err
	}
	if finder.ByID("jp.naver.line.android:id/chat_list_recycler_view") != nil {
		return true, nil
	}
	return false, nil
}

// DetectScreen determines which LINE screen is currently displayed.
func DetectScreen(finder *core.ElementFinder) Screen {
	if finder.ByID("jp.naver.line.android:id/chat_list_recycler_view") != nil {
		return ScreenChats
	}
	if finder.ByID("jp.naver.line.android:id/chathistory_message_edit_text") != nil {
		return ScreenChatDetail
	}
	return ScreenUnknown
}

// NavigateToChats navigates to the chat list.
func (b *LineBridge) NavigateToChats(ctx context.Context) error {
	const maxRetries = 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		finder, err := b.Workflow.FreshDump(ctx)
		if err != nil {
			return err
		}

		if DetectScreen(finder) == ScreenChats {
			return nil
		}

		chatsTab := finder.ByID("jp.naver.line.android:id/bnb_chat")
		if chatsTab == nil {
			chatsTab = finder.ByText("Chats", true)
		}
		if chatsTab != nil {
			c := chatsTab.Center()
			if err := b.Dev.Tap(ctx, c.X, c.Y); err != nil {
				return err
			}
			_, err := b.Workflow.WaitForElement(ctx, 5*time.Second,
				core.HasID("jp.naver.line.android:id/chat_list_recycler_view"))
			return err
		}

		if err := b.EnsureAppRunning(ctx); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
	}
	return core.ErrNavigation()
}
