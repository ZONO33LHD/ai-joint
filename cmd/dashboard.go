package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/shunsuke/ai-joint/internal/session"
	"github.com/shunsuke/ai-joint/internal/store"
	"github.com/shunsuke/ai-joint/internal/tui"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.New()
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		mgr, err := session.NewManager(st)
		if err != nil {
			return err
		}

		app := tui.NewApp(mgr, st)
		return app.Run()
	},
}
