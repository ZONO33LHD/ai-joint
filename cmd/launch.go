package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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

		// If a session with this name already exists, decide what to do.
		if existing := mgr.GetByName(name); existing != nil {
			if socketAlive(session.SocketPath(existing.ID)) {
				return fmt.Errorf("session %q is already running — use: aj attach %s", name, name)
			}
			// Dead socket (process crashed / old binary) — clean up and relaunch.
			fmt.Printf("session %q record exists but process is gone — reusing name\n", name)
			mgr.Delete(existing.ID)
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

		// Open log file so the dashboard can read PTY output across processes.
		logFile, err := os.OpenFile(session.OutputPath(s.ID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: open log file: %v\n", err)
		}
		defer func() {
			if logFile != nil {
				logFile.Close()
			}
		}()

		p, err := apty.Spawn(ctx, ccBin, dir, launchEnv,
			func(data []byte) {
				mgr.AppendOutput(s.ID, data)
				os.Stdout.Write(data)
				if logFile != nil {
					logFile.Write(data)
				}
			},
			func() {
				mgr.SetState(s.ID, session.StateDone)
				fmt.Fprintf(os.Stderr, "\nsession %q ended\n", name)
			},
		)
		if err != nil {
			return fmt.Errorf("spawn pty: %w", err)
		}

		// Set initial PTY size to match the current terminal window,
		// and persist it so the dashboard can replay with the same dimensions.
		if cols, rows, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			p.Resize(uint16(rows), uint16(cols))
			session.WriteSize(s.ID, cols, rows)
		}

		// Put stdin in raw mode so every keystroke is sent immediately.
		stdinFd := int(os.Stdin.Fd())
		if oldState, err := term.MakeRaw(stdinFd); err == nil {
			defer term.Restore(stdinFd, oldState)
		}

		// Track terminal resize signals and forward to the PTY.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGWINCH)
		go func() {
			for range sigCh {
				if cols, rows, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
					p.Resize(uint16(rows), uint16(cols))
				}
			}
		}()
		defer func() {
			signal.Stop(sigCh)
			close(sigCh)
		}()

		// Start Unix socket server so the dashboard can send input and resize commands.
		if err := p.StartInputServer(session.SocketPath(s.ID), func(cols, rows uint16) {
			session.WriteSize(s.ID, int(cols), int(rows))
		}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: input server: %v\n", err)
		}

		mgr.SetState(s.ID, session.StateIdle)

		// Forward stdin to the PTY.
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

// socketAlive returns true if something is actively listening on the Unix socket.
func socketAlive(path string) bool {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
