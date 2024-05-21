package data

import (
	"context"
	"fmt"

	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
)

type User struct {
	ID       string
	Username string
	Password []byte
}

func GetUserByUsername(db *sqlitex.Pool, username string, ctx context.Context) (*User, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	var user *User
	fn := func(stmt *sqlite.Stmt) error {
		user = &User{
			ID:       stmt.ColumnText(0),
			Username: stmt.ColumnText(1),
			Password: []byte(stmt.ColumnText(2)),
		}
		return nil
	}
	if err := sqlitex.Execute(conn,
		"SELECT id, username, password FROM users WHERE username = ?;", &sqlitex.ExecOptions{
			Args:       []any{username},
			ResultFunc: fn,
		}); err != nil {
		err = fmt.Errorf("unable to find user %s: %w", username, err)
		return nil, err
	}

	return user, nil
}
