package cmd

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/shopee"
	"github.com/prathan/pincer/src/pincer/drivers/shopee/commands"
	"github.com/spf13/cobra"
)

var shopeeCmd = &cobra.Command{
	Use:   "shopee",
	Short: "Shopee app commands (com.shopee.th)",
	Long: `Automate the Shopee e-commerce app.

Available domains:
  cart     View shopping cart contents
  search   Search for products`,
}

var shopeeCartCmd = &cobra.Command{
	Use:   "cart",
	Short: "Shopee cart commands",
	Example: `  pincer shopee cart list`,
}

var shopeeCartListCmd = &cobra.Command{
	Use:   "list",
	Short: "List items in Shopee shopping cart",
	Long: `List all items currently in the Shopee shopping cart. Each item includes
the shop name, product name, variation, current price, original price
(if discounted), and quantity.`,
	Example: `  pincer shopee cart list
  pincer shopee cart list | jq '.data.items[] | {name, price}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := shopee.NewShopeeDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.CartList(ctx, driver)
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
	Long: `Search for products by keyword. Navigates to the search bar, types the
query, and parses the results. Returns product names, prices, and
discount information.`,
	Example: `  pincer shopee search --query "usb-c cable"
  pincer shopee search -q "wireless charger" | jq '.data.products'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")

		if query == "" {
			outputError(core.NewDriverError("missing_argument", "--query is required"))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := shopee.NewShopeeDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.Search(ctx, driver, query)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

func init() {
	shopeeSearchCmd.Flags().StringP("query", "q", "", "Search query (required)")

	shopeeCartCmd.AddCommand(shopeeCartListCmd)
	shopeeCmd.AddCommand(shopeeCartCmd, shopeeSearchCmd)
	rootCmd.AddCommand(shopeeCmd)
}
