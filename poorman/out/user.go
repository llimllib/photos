package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/google/uuid"
)

type User struct {
	Id         string
	Username   string
	Password   string
	Createdat  time.Time
	Modifiedat time.Time
}

func NewUser(db *sqlitex.Pool, data *User, ctx context.Context) (*User, error) {
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
