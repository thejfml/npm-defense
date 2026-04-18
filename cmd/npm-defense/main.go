package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "v0.1.0-dev"
	commit  = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "npm-defense",
	Short: "Offline-first npm supply-chain attack detector",
	Long: `npm-defense scans your project's lockfile and flags dependencies
whose characteristics match known supply-chain attack patterns.

Zero telemetry. All analysis happens locally.`,
}

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a project's lockfile for suspicious packages",
	Long: `Scan reads package-lock.json from the specified path (default: current directory)
and applies detection rules to identify potentially malicious dependencies.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		// TODO: implement scan logic
		fmt.Fprintf(os.Stderr, "Scanning %s...\n", path)
		return fmt.Errorf("scan: not implemented yet")
	},
}

var explainCmd = &cobra.Command{
	Use:   "explain <rule-id>",
	Short: "Explain what a detection rule does",
	Long:  `Print detailed information about a specific detection rule: what it catches, what it misses, and its severity.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ruleID := args[0]

		// TODO: implement explain logic
		fmt.Fprintf(os.Stderr, "Explaining rule %s...\n", ruleID)
		return fmt.Errorf("explain: not implemented yet")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("npm-defense %s (commit: %s)\n", version, commit)
	},
}

func init() {
	// scan flags
	scanCmd.Flags().Bool("json", false, "Output findings as JSON")
	scanCmd.Flags().String("fail-on", "high", "Exit 1 if findings at this severity or above (low|medium|high)")
	scanCmd.Flags().Int("concurrency", 10, "Number of concurrent registry requests")
	scanCmd.Flags().Bool("offline", false, "Run in offline mode (cache only)")
	scanCmd.Flags().String("cache-dir", "", "Cache directory (default: $XDG_CACHE_HOME/npm-defense or ~/.cache/npm-defense)")
	scanCmd.Flags().Bool("no-cache", false, "Bypass cache for this run")

	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
