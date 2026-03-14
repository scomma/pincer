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
	Long: `Pincer automates Android apps via accessibility APIs, exposing
high-level domain commands as a CLI. Each supported app has a driver
that translates intent into UIAutomator sequences and returns
structured JSON.

Supported apps:
  grab     Grab (food delivery, transport)
  line     LINE (messaging)
  shopee   Shopee (e-commerce)

Output is always JSON to stdout. Errors are JSON to stderr with exit code 1.

Success: {"ok": true, "data": { ... }}
Error:   {"ok": false, "error": "code", "message": "..."}`,
	Example: `  pincer grab food search --query "pad thai"
  pincer line chat list --unread --limit 5
  pincer shopee cart list
  pincer grab auth status`,
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
	_ = enc.Encode(data)
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
