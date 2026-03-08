package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aj",
	Short: "ai-joint: Claude Code multi-session manager",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(launchCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(killCmd)
}
