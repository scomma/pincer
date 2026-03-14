package cmd

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/bridges/shopee"
	"github.com/prathan/pincer/src/pincer/bridges/shopee/commands"
	"github.com/prathan/pincer/src/pincer/core"
	"github.com/spf13/cobra"
)

var shopeeCmd = &cobra.Command{
	Use:   "shopee",
	Short: "Shopee app commands",
}

var shopeeCartCmd = &cobra.Command{
	Use:   "cart",
	Short: "Shopee cart commands",
}

var shopeeCartListCmd = &cobra.Command{
	Use:   "list",
	Short: "List items in Shopee shopping cart",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		bridge, err := shopee.NewShopeeBridge(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.CartList(ctx, bridge)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var shopeeSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for products on Shopee",
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")

		if query == "" {
			outputError(core.NewBridgeError("missing_argument", "--query is required"))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		bridge, err := shopee.NewShopeeBridge(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.Search(ctx, bridge, query)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

func init() {
	shopeeSearchCmd.Flags().StringP("query", "q", "", "Search query")

	shopeeCartCmd.AddCommand(shopeeCartListCmd)
	shopeeCmd.AddCommand(shopeeCartCmd, shopeeSearchCmd)
	rootCmd.AddCommand(shopeeCmd)
}
