package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	apty "github.com/shunsuke/ai-joint/internal/pty"
	"github.com/shunsuke/ai-joint/internal/session"
	"github.com/shunsuke/ai-joint/internal/store"
)

var (
	launchDir string
	launchEnv []string
	ccBin     string
)

var launchCmd = &cobra.Command{
	Use:   "launch [name]",
	Short: "Launch a new Claude Code session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		name := args[0]

		st, err := store.New()
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		mgr, err := session.NewManager(st)
		if err != nil {
			return err
		}

		dir := launchDir
		if dir == "" {
			if dir, err = os.Getwd(); err != nil {
				return fmt.Errorf("get working dir: %w", err)
			}
		}

		id, err := newID()
		if err != nil {
			return err
		}

		s, err := mgr.Create(id, name, dir)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		fmt.Printf("Launching session %q (id=%s) in %s\n", name, s.ID, dir)

		p, err := apty.Spawn(ctx, ccBin, dir, launchEnv,
			func(data []byte) {
				mgr.AppendOutput(s.ID, data)
				os.Stdout.Write(data)
			},
			func() {
				mgr.SetState(s.ID, session.StateDone)
				fmt.Fprintf(os.Stderr, "\nsession %q ended\n", name)
			},
		)
		if err != nil {
			return fmt.Errorf("spawn pty: %w", err)
		}

		mgr.SetState(s.ID, session.StateIdle)

		// Forward stdin to PTY; exits when the PTY closes or stdin reaches EOF.
		go func() {
			io.Copy(p, os.Stdin)
		}()

		return p.Wait()
	},
}

func init() {
	launchCmd.Flags().StringVarP(&launchDir, "dir", "d", "", "working directory (default: cwd)")
	launchCmd.Flags().StringArrayVarP(&launchEnv, "env", "e", nil, "extra env vars (KEY=VAL)")
	launchCmd.Flags().StringVar(&ccBin, "cc", "claude", "path to claude binary")
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
