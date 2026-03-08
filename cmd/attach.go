package cmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"github.com/shunsuke/ai-joint/internal/session"
	"github.com/shunsuke/ai-joint/internal/store"
)

var attachCmd = &cobra.Command{
	Use:   "attach [name]",
	Short: "Attach to a running session (re-display output + forward input)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		st, err := store.New()
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		row, err := st.GetSessionByName(name)
		if err != nil {
			return err
		}
		if row == nil {
			return fmt.Errorf("session %q not found — run: aj ls", name)
		}
		if row.State == "done" {
			return fmt.Errorf("session %q has already ended", name)
		}

		// Connect to the PTY input socket.
		conn, err := net.Dial("unix", session.SocketPath(row.ID))
		if err != nil {
			return fmt.Errorf("session not running (socket unavailable): %w\nhint: restart with: aj launch %s", err, name)
		}
		defer conn.Close()

		// Replay existing log output so the current terminal shows the session state.
		logPath := session.OutputPath(row.ID)
		logFile, err := os.Open(logPath)
		if err != nil {
			return fmt.Errorf("open session log: %w", err)
		}
		io.Copy(os.Stdout, logFile)
		offset, _ := logFile.Seek(0, io.SeekCurrent)
		logFile.Close()

		// Put stdin in raw mode.
		stdinFd := int(os.Stdin.Fd())
		if oldState, err := term.MakeRaw(stdinFd); err == nil {
			defer term.Restore(stdinFd, oldState)
		}

		// Send resize events when the terminal window changes.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGWINCH)
		go func() {
			for range sigCh {
				// Resize is handled by the aj launch process which owns the PTY.
				// We just re-read our own size for reference; the PTY owner resizes itself.
			}
		}()
		defer func() {
			signal.Stop(sigCh)
			close(sigCh)
		}()

		// Forward stdin → socket.
		go func() {
			io.Copy(conn, os.Stdin)
		}()

		// Tail the log file and stream new bytes to stdout.
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			f, err := os.Open(logPath)
			if err != nil {
				continue
			}
			f.Seek(offset, io.SeekStart)
			n, _ := io.Copy(os.Stdout, f)
			offset += n
			f.Close()

			// Stop tailing when the session ends.
			if row2, _ := st.GetSessionByName(name); row2 != nil && row2.State == "done" {
				return nil
			}
		}
		return nil
	},
}
