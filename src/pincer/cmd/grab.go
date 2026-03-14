package cmd

import (
	"context"
	"time"

	"github.com/prathan/pincer/src/pincer/bridges/grab"
	"github.com/prathan/pincer/src/pincer/bridges/grab/commands"
	"github.com/prathan/pincer/src/pincer/core"
	"github.com/spf13/cobra"
)

var grabCmd = &cobra.Command{
	Use:   "grab",
	Short: "Grab app commands",
}

var grabFoodCmd = &cobra.Command{
	Use:   "food",
	Short: "Grab Food commands",
}

var grabFoodSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for restaurants on Grab Food",
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		bridge, err := grab.NewGrabBridge(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.FoodSearch(ctx, bridge, query)
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
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		bridge, err := grab.NewGrabBridge(newADB())
		if err != nil {
			outputError(err)
			return nil
		}

		result, err := commands.AuthStatus(ctx, bridge)
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
