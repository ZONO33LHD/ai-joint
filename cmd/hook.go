package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/shunsuke/ai-joint/internal/store"
	"github.com/shunsuke/ai-joint/internal/tracker"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Process a Claude Code hook event from stdin",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}

		st, err := store.New()
		if err != nil {
			return err
		}
		defer st.Close()

		t := tracker.New(st)
		if err := t.Process(data); err != nil {
			return fmt.Errorf("process hook: %w", err)
		}
		return nil
	},
}
