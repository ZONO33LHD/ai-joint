package tracker

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shunsuke/ai-joint/internal/store"
)

// HookEvent represents the JSON payload from Claude Code hooks.
type HookEvent struct {
	Event         string `json:"event"`
	SessionID     string `json:"session_id"`
	ToolName      string `json:"tool_name"`
	CustomCommand string `json:"custom_command"`
	RulesFile     string `json:"rules_file"`
	AgentName     string `json:"agent_name"`
}

type Tracker struct {
	store *store.Store
}

func New(st *store.Store) *Tracker {
	return &Tracker{store: st}
}

// Process parses a JSON hook payload and records relevant activities.
func (t *Tracker) Process(data []byte) error {
	var ev HookEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return fmt.Errorf("parse hook event: %w", err)
	}

	now := time.Now()

	if ev.ToolName != "" {
		if err := t.store.InsertActivity(store.ActivityRow{
			SessionID:  ev.SessionID,
			Kind:       "tool",
			Value:      ev.ToolName,
			OccurredAt: now,
		}); err != nil {
			return err
		}
	}

	if ev.CustomCommand != "" {
		if err := t.store.InsertActivity(store.ActivityRow{
			SessionID:  ev.SessionID,
			Kind:       "cmd",
			Value:      ev.CustomCommand,
			OccurredAt: now,
		}); err != nil {
			return err
		}
	}

	if ev.RulesFile != "" {
		if err := t.store.InsertActivity(store.ActivityRow{
			SessionID:  ev.SessionID,
			Kind:       "rule",
			Value:      ev.RulesFile,
			OccurredAt: now,
		}); err != nil {
			return err
		}
	}

	if ev.AgentName != "" {
		if err := t.store.InsertActivity(store.ActivityRow{
			SessionID:  ev.SessionID,
			Kind:       "agent",
			Value:      ev.AgentName,
			OccurredAt: now,
		}); err != nil {
			return err
		}
	}

	return nil
}
