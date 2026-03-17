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
  order    View and reorder past orders
  search   Search for products`,
}

var shopeeCartCmd = &cobra.Command{
	Use:   "cart",
	Short: "Shopee cart commands",
	Example: `  pincer shopee cart list
  pincer shopee cart update --item "COCOFON Organic Toilet" --quantity 3
  pincer shopee cart remove --item "COCOFON Organic Toilet"
  pincer shopee cart checkout --max-total 100`,
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

var shopeeCartUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update quantity of a cart item",
	Long: `Update the quantity of an item in the Shopee shopping cart.
Finds the item by name (case-insensitive substring match) and taps the
+/- buttons to reach the target quantity.`,
	Example: `  pincer shopee cart update --item "COCOFON Organic Toilet" --quantity 3`,
	RunE: func(cmd *cobra.Command, args []string) error {
		item, _ := cmd.Flags().GetString("item")
		quantity, _ := cmd.Flags().GetInt("quantity")

		if item == "" {
			outputError(core.NewDriverError("missing_argument", "--item is required"))
			return nil
		}
		if quantity < 1 {
			outputError(core.NewDriverError("invalid_argument", "--quantity must be at least 1"))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := shopee.NewShopeeDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.CartUpdate(ctx, driver, item, quantity)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var shopeeCartRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove an item from the cart",
	Long: `Remove an item from the Shopee shopping cart. Finds the item by name
(case-insensitive substring match), enters edit mode for the item's shop
section, and taps the Delete button.`,
	Example: `  pincer shopee cart remove --item "COCOFON Organic Toilet"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		item, _ := cmd.Flags().GetString("item")

		if item == "" {
			outputError(core.NewDriverError("missing_argument", "--item is required"))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := shopee.NewShopeeDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.CartRemove(ctx, driver, item)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var shopeeCartCheckoutCmd = &cobra.Command{
	Use:   "checkout",
	Short: "Select all cart items, go to checkout, and return the quotation",
	Long: `Select all items in the cart, proceed to the checkout page, parse the
order quotation (totals, shipping, vouchers), and return it as JSON.
Presses Back to exit the checkout page after parsing.`,
	Example: `  pincer shopee cart checkout
  pincer shopee cart checkout | jq '.data.total'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := shopee.NewShopeeDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.CartCheckout(ctx, driver)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var shopeeOrderCmd = &cobra.Command{
	Use:   "order",
	Short: "Shopee order commands",
	Example: `  pincer shopee order list
  pincer shopee order list --status completed --limit 3
  pincer shopee order reorder --item "SONY"`,
}

var shopeeOrderListCmd = &cobra.Command{
	Use:   "list",
	Short: "List past orders from My Purchases",
	Long: `List orders from the Shopee My Purchases page. Each order includes
the shop name, order status, and items with name, variation, quantity,
and price.`,
	Example: `  pincer shopee order list
  pincer shopee order list --status completed
  pincer shopee order list --limit 3 | jq '.data.orders[] | {shop, status}'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		limit, _ := cmd.Flags().GetInt("limit")

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := shopee.NewShopeeDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.OrderList(ctx, driver, status, limit)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var shopeeOrderReorderCmd = &cobra.Command{
	Use:   "reorder",
	Short: "Re-add a past order item to cart via Buy Again",
	Long: `Find a completed order containing the specified item (case-insensitive
substring match) and tap the "Buy Again" button. If a variant/quantity
picker appears, it will be confirmed automatically.`,
	Example: `  pincer shopee order reorder --item "SONY"
  pincer shopee order reorder -i "wireless charger"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		item, _ := cmd.Flags().GetString("item")

		if item == "" {
			outputError(core.NewDriverError("missing_argument", "--item is required"))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := shopee.NewShopeeDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.OrderReorder(ctx, driver, item)
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

	shopeeCartUpdateCmd.Flags().StringP("item", "i", "", "Item name to search for (required)")
	shopeeCartUpdateCmd.Flags().IntP("quantity", "q", 0, "Target quantity (required)")

	shopeeCartRemoveCmd.Flags().StringP("item", "i", "", "Item name to search for (required)")

	shopeeOrderListCmd.Flags().StringP("status", "s", "", "Filter by status tab (completed, to ship, to receive, cancelled)")
	shopeeOrderListCmd.Flags().IntP("limit", "l", 5, "Maximum number of orders to return")

	shopeeOrderReorderCmd.Flags().StringP("item", "i", "", "Item name to search for (required)")

	shopeeOrderCmd.AddCommand(shopeeOrderListCmd, shopeeOrderReorderCmd)
	shopeeCartCmd.AddCommand(shopeeCartListCmd, shopeeCartUpdateCmd, shopeeCartRemoveCmd, shopeeCartCheckoutCmd)
	shopeeCmd.AddCommand(shopeeCartCmd, shopeeOrderCmd, shopeeSearchCmd)
	rootCmd.AddCommand(shopeeCmd)
}
