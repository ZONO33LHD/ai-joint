// Package hooks manages Claude Code hook configuration for ai-joint.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ajHookCommand returns the command string for the aj hook.
func ajHookCommand() string {
	// Use the absolute path of the running binary so hooks work regardless of PATH.
	if exe, err := os.Executable(); err == nil {
		return exe + " hook"
	}
	return "aj hook"
}

// hookEntry represents a single hook in Claude Code settings.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

// EnsureHooks adds "aj hook" to Claude Code's global settings if not already present.
// It adds hooks to PostToolUse, SubagentStop, and Stop events.
func EnsureHooks() error {
	settingsPath := filepath.Join(os.Getenv("HOME"), ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read settings: %w", err)
	}

	var settings map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	} else {
		settings = make(map[string]any)
	}

	hooksRaw, _ := settings["hooks"]
	hooks, _ := hooksRaw.(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	ajCmd := ajHookCommand()
	changed := false

	// Events where we want to record activity.
	events := []string{"PostToolUse", "SubagentStop", "Stop"}
	for _, event := range events {
		if ensureEventHook(hooks, event, ajCmd) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	fmt.Println("✓ Added aj hook to Claude Code settings")
	return nil
}

// ensureEventHook ensures the aj hook command exists in the given event's hook list.
// Returns true if it was added.
func ensureEventHook(hooks map[string]any, event, ajCmd string) bool {
	groupsRaw, _ := hooks[event]
	groups, _ := groupsRaw.([]any)

	// Check if aj hook already exists in any group.
	for _, g := range groups {
		gm, _ := g.(map[string]any)
		if gm == nil {
			continue
		}
		hooksArr, _ := gm["hooks"].([]any)
		for _, h := range hooksArr {
			hm, _ := h.(map[string]any)
			cmd, _ := hm["command"].(string)
			if strings.Contains(cmd, "aj hook") || strings.Contains(cmd, "ai-joint hook") {
				return false // already configured
			}
		}
	}

	// Add a new hook group with wildcard matcher.
	newGroup := map[string]any{
		"matcher": "*",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": ajCmd,
			},
		},
	}
	hooks[event] = append(groups, newGroup)
	return true
}

// AJBinaryPath returns the absolute path of the aj binary.
// Falls back to looking up "aj" in PATH.
func AJBinaryPath() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	if p, err := exec.LookPath("aj"); err == nil {
		return p
	}
	return "aj"
}
