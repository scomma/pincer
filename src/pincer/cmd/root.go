package cmd

import (
	"encoding/json"
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
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 90, "Command timeout in seconds")
}

func newADB() *core.ADB {
	return core.NewADB(deviceID)
}

func outputJSONTo(stream *os.File, data any) {
	enc := json.NewEncoder(stream)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

func outputJSON(data any) {
	outputJSONTo(os.Stdout, data)
}

func outputError(err error) {
	if be, ok := err.(*core.DriverError); ok {
		outputJSONTo(os.Stderr, core.NewErrorResponse(be))
	} else {
		outputJSONTo(os.Stderr, core.ErrorResponse{
			OK:      false,
			Error:   "internal_error",
			Message: err.Error(),
		})
	}
	os.Exit(1)
}
