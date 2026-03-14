package cmd

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/drivers/line"
	"github.com/prathan/pincer/src/pincer/drivers/line/commands"
	"github.com/prathan/pincer/src/pincer/core"
	"github.com/spf13/cobra"
)

var lineCmd = &cobra.Command{
	Use:   "line",
	Short: "LINE app commands",
}

var lineChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "LINE chat commands",
}

var lineChatListCmd = &cobra.Command{
	Use:   "list",
	Short: "List LINE chats",
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

func init() {
	lineChatListCmd.Flags().Bool("unread", false, "Show only unread chats")
	lineChatListCmd.Flags().IntP("limit", "n", 0, "Limit number of results")

	lineChatReadCmd.Flags().String("chat", "", "Chat name to read")
	lineChatReadCmd.Flags().IntP("limit", "n", 20, "Limit number of messages")

	lineChatCmd.AddCommand(lineChatListCmd, lineChatReadCmd)
	lineCmd.AddCommand(lineChatCmd)
	rootCmd.AddCommand(lineCmd)
}
