package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/google/uuid"
)

type Session_Data struct{ Username string }

type Session struct {
	Id        string
	Data      Session_Data
	Createdat time.Time
}

func NewSession(db *sqlitex.Pool, data *Session, ctx context.Context) (*Session, error) {
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
