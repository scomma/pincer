package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/prathan/pincer/src/pincer/core"
	"github.com/spf13/cobra"
)

var (
	deviceID string
	timeout  int
)

var rootCmd = &cobra.Command{
	Use:   "pincer",
	Short: "App driver framework for Android automation",
	Long:  "Pincer automates Android apps via accessibility APIs, exposing high-level domain commands as a CLI.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&deviceID, "device", "d", "", "ADB device serial (default: auto-detect)")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 30, "Command timeout in seconds")
}

func newADB() *core.ADB {
	return core.NewADB(deviceID)
}

func outputJSON(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func outputError(err error) {
	if be, ok := err.(*core.DriverError); ok {
		outputJSON(core.NewErrorResponse(be))
	} else {
		outputJSON(core.ErrorResponse{
			OK:      false,
			Error:   "internal_error",
			Message: err.Error(),
		})
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
