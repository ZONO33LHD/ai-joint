package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type SessionRow struct {
	ID        string
	Name      string
	Dir       string
	State     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ActivityRow struct {
	ID         int64
	SessionID  string
	Kind       string
	Value      string
	OccurredAt time.Time
}

func New() (*Store, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "ai-joint")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dir, "ai-joint.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) UpsertSession(row SessionRow) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (id, name, dir, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			dir=excluded.dir,
			state=excluded.state,
			updated_at=excluded.updated_at
	`, row.ID, row.Name, row.Dir, row.State,
		row.CreatedAt.Unix(), row.UpdatedAt.Unix())
	return err
}

func (s *Store) UpdateSessionState(id, state string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET state=?, updated_at=? WHERE id=?`,
		state, time.Now().Unix(), id,
	)
	return err
}

func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id=?`, id)
	return err
}

func (s *Store) ListSessions() ([]SessionRow, error) {
	rows, err := s.db.Query(
		`SELECT id, name, dir, state, created_at, updated_at FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SessionRow
	for rows.Next() {
		var (
			r                  SessionRow
			createdAt, updatedAt int64
		)
		if err := rows.Scan(&r.ID, &r.Name, &r.Dir, &r.State, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(createdAt, 0)
		r.UpdatedAt = time.Unix(updatedAt, 0)
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) GetSession(id string) (*SessionRow, error) {
	var (
		r                  SessionRow
		createdAt, updatedAt int64
	)
	err := s.db.QueryRow(
		`SELECT id, name, dir, state, created_at, updated_at FROM sessions WHERE id=?`, id,
	).Scan(&r.ID, &r.Name, &r.Dir, &r.State, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = time.Unix(createdAt, 0)
	r.UpdatedAt = time.Unix(updatedAt, 0)
	return &r, nil
}

func (s *Store) GetSessionByName(name string) (*SessionRow, error) {
	var (
		r                  SessionRow
		createdAt, updatedAt int64
	)
	err := s.db.QueryRow(
		`SELECT id, name, dir, state, created_at, updated_at FROM sessions WHERE name=?`, name,
	).Scan(&r.ID, &r.Name, &r.Dir, &r.State, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = time.Unix(createdAt, 0)
	r.UpdatedAt = time.Unix(updatedAt, 0)
	return &r, nil
}

func (s *Store) InsertActivity(row ActivityRow) error {
	_, err := s.db.Exec(`
		INSERT INTO activities (session_id, kind, value, occurred_at)
		VALUES (?, ?, ?, ?)
	`, row.SessionID, row.Kind, row.Value, row.OccurredAt.Unix())
	return err
}

func (s *Store) ListActivities(limit int) ([]ActivityRow, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, kind, value, occurred_at FROM activities ORDER BY occurred_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActivityRow
	for rows.Next() {
		var (
			r          ActivityRow
			occurredAt int64
		)
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Kind, &r.Value, &occurredAt); err != nil {
			return nil, err
		}
		r.OccurredAt = time.Unix(occurredAt, 0)
		result = append(result, r)
	}
	return result, rows.Err()
}
