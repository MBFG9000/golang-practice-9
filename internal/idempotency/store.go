package idempotency

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

const (
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
)

type Record struct {
	Key          string
	Status       string
	ResponseCode int
	ResponseBody string
}

type Store struct {
	db *sqlx.DB
}

func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("idempotency store is not initialized")
	}

	schema := `
CREATE TABLE IF NOT EXISTS idempotency_keys (
    key TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    response_code INTEGER,
    response_body TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) TryStart(ctx context.Context, key string) (bool, *Record, error) {
	if s == nil || s.db == nil {
		return false, nil, errors.New("idempotency store is not initialized")
	}

	insert := `INSERT INTO idempotency_keys (key, status) VALUES (?, ?) ON CONFLICT (key) DO NOTHING`
	result, err := s.db.ExecContext(ctx, s.db.Rebind(insert), key, StatusProcessing)
	if err != nil {
		return false, nil, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, nil, err
	}
	if affected > 0 {
		return true, nil, nil
	}

	record, err := s.Get(ctx, key)
	return false, record, err
}

func (s *Store) Get(ctx context.Context, key string) (*Record, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("idempotency store is not initialized")
	}

	query := `SELECT key, status, response_code, response_body FROM idempotency_keys WHERE key = ?`
	var row struct {
		Key          string         `db:"key"`
		Status       string         `db:"status"`
		ResponseCode sql.NullInt64  `db:"response_code"`
		ResponseBody sql.NullString `db:"response_body"`
	}

	if err := s.db.GetContext(ctx, &row, s.db.Rebind(query), key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	record := &Record{
		Key:    row.Key,
		Status: row.Status,
	}
	if row.ResponseCode.Valid {
		record.ResponseCode = int(row.ResponseCode.Int64)
	}
	if row.ResponseBody.Valid {
		record.ResponseBody = row.ResponseBody.String
	}

	return record, nil
}

func (s *Store) Complete(ctx context.Context, key string, statusCode int, responseBody []byte) error {
	if s == nil || s.db == nil {
		return errors.New("idempotency store is not initialized")
	}

	update := `
UPDATE idempotency_keys
SET status = ?, response_code = ?, response_body = ?, updated_at = CURRENT_TIMESTAMP
WHERE key = ?`
	_, err := s.db.ExecContext(ctx, s.db.Rebind(update), StatusCompleted, statusCode, string(responseBody), key)
	return err
}
