package cmd

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/prathan/pincer/src/pincer/drivers/grab"
	"github.com/prathan/pincer/src/pincer/drivers/grab/commands"
	"github.com/spf13/cobra"
)

var grabCmd = &cobra.Command{
	Use:   "grab",
	Short: "Grab app commands (com.grabtaxi.passenger)",
	Long: `Automate the Grab app (food delivery, transport, payments).

Available domains:
  food   Browse restaurants and search for food
  auth   Check login status`,
}

var grabFoodCmd = &cobra.Command{
	Use:   "food",
	Short: "Grab Food — browse and search restaurants",
	Long: `Commands for interacting with Grab's food delivery service.
Parses restaurant cards including name, promo text, and ratings.`,
	Example: `  pincer grab food search
  pincer grab food search --query "pad thai"`,
}

var grabFoodSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for restaurants on Grab Food",
	Long: `List visible restaurants on the Grab Food home screen. If --query is
provided, types it into the search bar and returns matching results.
Without --query, returns the default restaurant listing.

Output includes restaurant name and active promotions.`,
	Example: `  # Browse default listings
  pincer grab food search

  # Search for specific food
  pincer grab food search --query "pad thai"

  # Pipe to jq for filtering
  pincer grab food search | jq '.data.restaurants[:3]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := grab.NewGrabDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.FoodSearch(ctx, driver, query)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

var grabAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Grab authentication commands",
}

var grabAuthStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Grab login status",
	Long: `Check whether the user is logged in to the Grab app.
Returns the current screen name and login state.

Possible screens: HOME, FOOD_HOME, FOOD_RESULTS, LOGIN_PHONE, LOGIN_OTP, LOGIN_PIN, UNKNOWN.`,
	Example: `  pincer grab auth status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		driver, err := grab.NewGrabDriver(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.AuthStatus(ctx, driver)
		if err != nil {
			outputError(err)
			return nil
		}

		outputJSON(core.NewResponse(result))
		return nil
	},
}

func init() {
	grabFoodSearchCmd.Flags().StringP("query", "q", "", "Search query")

	grabFoodCmd.AddCommand(grabFoodSearchCmd)
	grabAuthCmd.AddCommand(grabAuthStatusCmd)
	grabCmd.AddCommand(grabFoodCmd, grabAuthCmd)
	rootCmd.AddCommand(grabCmd)
}
