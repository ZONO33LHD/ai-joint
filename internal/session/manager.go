package session

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/shunsuke/ai-joint/internal/store"
)

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	store    *store.Store
	onChange func()
}

func NewManager(st *store.Store) (*Manager, error) {
	m := &Manager{
		sessions: make(map[string]*Session),
		store:    st,
	}
	if err := m.loadFromStore(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) SetOnChange(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

func (m *Manager) loadFromStore() error {
	rows, err := m.store.ListSessions()
	if err != nil {
		return err
	}
	for _, r := range rows {
		s := &Session{
			ID:        r.ID,
			Name:      r.Name,
			Dir:       r.Dir,
			State:     State(r.State),
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		}
		m.sessions[s.ID] = s
	}
	return nil
}

func (m *Manager) Create(id, name, dir string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	s := &Session{
		ID:        id,
		Name:      name,
		Dir:       dir,
		State:     StateIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := m.store.UpsertSession(store.SessionRow{
		ID:        s.ID,
		Name:      s.Name,
		Dir:       s.Dir,
		State:     string(s.State),
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}); err != nil {
		return nil, fmt.Errorf("store session: %w", err)
	}
	m.sessions[s.ID] = s
	m.notifyChange()
	return s, nil
}

func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

func (m *Manager) GetByName(name string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Collect(maps.Values(m.sessions))
}

func (m *Manager) SetState(id string, state State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	s.State = state
	s.UpdatedAt = time.Now()
	if err := m.store.UpdateSessionState(id, string(state)); err != nil {
		return fmt.Errorf("update state: %w", err)
	}
	m.notifyChange()
	return nil
}

func (m *Manager) AppendOutput(id string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return
	}
	s.Output = append(s.Output, data...)
	// Keep last 64 KiB to bound memory usage.
	const maxBuf = 64 << 10
	if len(s.Output) > maxBuf {
		s.Output = s.Output[len(s.Output)-maxBuf:]
	}
	m.notifyChange()
}

// Reload re-reads all sessions from the store. Call this to pick up changes
// made by external processes (e.g. hook subcommands).
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rows, err := m.store.ListSessions()
	if err != nil {
		return err
	}
	next := make(map[string]*Session, len(rows))
	for _, r := range rows {
		var s *Session
		if existing, ok := m.sessions[r.ID]; ok {
			existing.Name = r.Name
			existing.Dir = r.Dir
			existing.State = State(r.State)
			existing.UpdatedAt = r.UpdatedAt
			s = existing
		} else {
			s = &Session{
				ID:        r.ID,
				Name:      r.Name,
				Dir:       r.Dir,
				State:     State(r.State),
				CreatedAt: r.CreatedAt,
				UpdatedAt: r.UpdatedAt,
			}
		}
		// Sync output buffer from log file so the dashboard always has
		// the latest PTY output even though it runs in a separate process.
		if data, err := readLastN(OutputPath(r.ID), 64<<10); err == nil {
			s.Output = data
		}
		next[r.ID] = s
	}
	m.sessions = next
	return nil
}

// readLastN reads up to n bytes from the end of the file at path.
func readLastN(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	offset := fi.Size() - n
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session %q not found", id)
	}
	if err := m.store.DeleteSession(id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	delete(m.sessions, id)
	m.notifyChange()
	return nil
}

// notifyChange fires the onChange callback asynchronously so callers holding
// the lock are not blocked waiting on TUI redraws.
func (m *Manager) notifyChange() {
	if m.onChange != nil {
		go m.onChange()
	}
}
