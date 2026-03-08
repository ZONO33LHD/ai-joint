package session

import (
	"fmt"
	"maps"
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
