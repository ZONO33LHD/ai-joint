package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/shunsuke/ai-joint/internal/session"
	"github.com/shunsuke/ai-joint/internal/store"
)

var killCmd = &cobra.Command{
	Use:   "kill [name]",
	Short: "Remove a session record by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		st, err := store.New()
		if err != nil {
			return err
		}
		defer st.Close()

		mgr, err := session.NewManager(st)
		if err != nil {
			return err
		}

		s := mgr.GetByName(name)
		if s == nil {
			return fmt.Errorf("session %q not found", name)
		}

		if err := mgr.Delete(s.ID); err != nil {
			return err
		}
		fmt.Printf("session %q removed\n", name)
		return nil
	},
}
