package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/google/uuid"
)

type Session struct {
	ID        string
	Data      *SessionData
	CreatedAt time.Time
}

type SessionData struct {
	Username string `json:"string"`
}

func NewSession(db *sqlitex.Pool, data *SessionData, ctx context.Context) (*Session, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	createdAt := time.Now()
	id, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}
	encData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	if err := sqlitex.Execute(conn,
		"INSERT INTO sessions (id, data, created_at) VALUES (?, ?, ?);",
		&sqlitex.ExecOptions{
			Args: []any{id, encData, createdAt.Format(time.RFC3339)},
		}); err != nil {
		return nil, err
	}

	return &Session{
		ID:        id.String(),
		Data:      data,
		CreatedAt: createdAt,
	}, nil
}

func LookupSession(db *sqlitex.Pool, id string, ctx context.Context) (*Session, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	var session *Session
	fn := func(stmt *sqlite.Stmt) error {
		t, err := time.Parse(time.RFC3339, stmt.ColumnText(2))
		if err != nil {
			return err
		}

		var data SessionData
		err = json.Unmarshal([]byte(stmt.ColumnText(1)), &data)
		if err != nil {
			return err
		}

		session = &Session{
			ID:        stmt.ColumnText(0),
			Data:      &data,
			CreatedAt: t,
		}
		return nil
	}
	if err := sqlitex.Execute(conn,
		"SELECT id, data, created_at FROM sessions WHERE id = ?;",
		&sqlitex.ExecOptions{
			ResultFunc: fn,
			Args:       []any{id},
		}); err != nil {
		return nil, fmt.Errorf("unable to find session %s: %w", id, err)
	}

	return session, nil
}
