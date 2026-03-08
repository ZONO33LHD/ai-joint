package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/shunsuke/ai-joint/internal/store"
)

var (
	lsJSON     bool
	lsNoHeader bool
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.New()
		if err != nil {
			return err
		}
		defer st.Close()

		sessions, err := st.ListSessions()
		if err != nil {
			return err
		}

		if lsJSON {
			return json.NewEncoder(os.Stdout).Encode(sessions)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if !lsNoHeader {
			fmt.Fprintln(w, "NAME\tSTATE\tDIR\tUPDATED")
		}
		for _, s := range sessions {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				s.Name, s.State, s.Dir, s.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
		return w.Flush()
	},
}

func init() {
	lsCmd.Flags().BoolVar(&lsJSON, "json", false, "output as JSON")
	lsCmd.Flags().BoolVar(&lsNoHeader, "no-header", false, "suppress header")
}
