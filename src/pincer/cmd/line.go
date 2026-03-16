package cmd

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/line"
	"github.com/prathan/pincer/src/pincer/drivers/line/commands"
	"github.com/spf13/cobra"
)

var lineCmd = &cobra.Command{
	Use:   "line",
	Short: "LINE app commands (jp.naver.line.android)",
	Long: `Automate the LINE messaging app.

Available domains:
  chat   List conversations, read messages`,
}

var lineChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "LINE chat commands — list, read, and send messages",
	Long: `Commands for interacting with LINE chats. Parses chat names, last messages,
timestamps, unread counts, and member counts.`,
	Example: `  pincer line chat list
  pincer line chat list --unread --limit 5
  pincer line chat read --chat "Family Direct"
  pincer line chat send --chat "Keep Memo" --message "hello"`,
}

var lineChatListCmd = &cobra.Command{
	Use:   "list",
	Short: "List LINE chats",
	Long: `List visible chats from the LINE chat tab. Each entry includes the chat
name, last message preview, timestamp, unread count, member count, and
muted status.

Use --unread to filter to only chats with unread messages.
Use --limit to cap the number of results.`,
	Example: `  # List all visible chats
  pincer line chat list

  # Only unread, top 5
  pincer line chat list --unread --limit 5

  # Pipe to jq
  pincer line chat list | jq '.data.chats[] | select(.unread_count > 100)'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		unread, _ := cmd.Flags().GetBool("unread")
		limit, _ := cmd.Flags().GetInt("limit")

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := line.NewLineDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.ChatList(ctx, driver, unread, limit)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var lineChatReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read messages from a LINE chat",
	Long: `Open a specific chat by name and read its messages. The --chat flag
must match the chat name exactly (case-sensitive). Use "pincer line
chat list" first to discover available chat names.

Returns messages with text content. Use --limit to control how many
messages are returned (most recent N).`,
	Example: `  # Read last 20 messages from "Family Direct"
  pincer line chat read --chat "Family Direct"

  # Read last 5 messages
  pincer line chat read --chat "Project Atlas" --limit 5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		chatName, _ := cmd.Flags().GetString("chat")
		limit, _ := cmd.Flags().GetInt("limit")

		if chatName == "" {
			outputError(core.NewDriverError("missing_argument", "--chat is required"))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := line.NewLineDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.ChatRead(ctx, driver, chatName, limit)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var lineChatSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a message or location to a LINE chat",
	Long: `Send a text message or share a location to a specific chat by name.
The --chat flag must match the chat name exactly (case-sensitive).

Use --message for text, or --location for locations. Use --location
"current" to share the device's current GPS position, or pass a place
name to search and share.

For safe testing, use "Keep Memo" which is LINE's note-to-self.`,
	Example: `  # Send a text message
  pincer line chat send --chat "Keep Memo" --message "hello from pincer"

  # Share current location
  pincer line chat send --chat "Keep Memo" --location current

  # Share a specific place
  pincer line chat send --chat "Keep Memo" --location "centralwOrld"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		chatName, _ := cmd.Flags().GetString("chat")
		message, _ := cmd.Flags().GetString("message")
		location, _ := cmd.Flags().GetString("location")

		if chatName == "" {
			outputError(core.NewDriverError("missing_argument", "--chat is required"))
			return nil
		}
		if message == "" && location == "" {
			outputError(core.NewDriverError("missing_argument", "--message or --location is required"))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := line.NewLineDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		if location != "" {
			result, err := commands.ChatSendLocation(ctx, driver, chatName, location)
			if err != nil {
				outputError(err)
				return nil
			}
			outputJSON(core.NewResponse(result))
			return nil
		}

		result, err := commands.ChatSend(ctx, driver, chatName, message)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

func init() {
	lineChatListCmd.Flags().Bool("unread", false, "Show only unread chats")
	lineChatListCmd.Flags().IntP("limit", "n", 0, "Limit number of results (0 = no limit)")

	lineChatReadCmd.Flags().String("chat", "", "Chat name to read (required, exact match)")
	lineChatReadCmd.Flags().IntP("limit", "n", 20, "Limit number of messages")

	lineChatSendCmd.Flags().String("chat", "", "Chat name to send to (required, exact match)")
	lineChatSendCmd.Flags().StringP("message", "m", "", "Text message to send")
	lineChatSendCmd.Flags().StringP("location", "l", "", "Location to share: 'current' or a place name to search")

	lineChatCmd.AddCommand(lineChatListCmd, lineChatReadCmd, lineChatSendCmd)
	lineCmd.AddCommand(lineChatCmd)
	rootCmd.AddCommand(lineCmd)
}
