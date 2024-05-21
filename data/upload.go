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

type UploadMetadata struct {
	Iso      string
	Camera   string
	Aperture string
	Exposure string
}

type Upload struct {
	ID         string
	Title      string
	Caption    string
	Filename   string
	Metadata   UploadMetadata
	CreatedAt  time.Time
	ModifiedAt time.Time
}

func NewUpload(
	db *sqlitex.Pool,
	title, caption, filename string,
	metadata UploadMetadata,
	ctx context.Context,
) (*Upload, error) {
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
	encMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	if err := sqlitex.Execute(conn,
		`INSERT INTO uploads (id, title, caption, filename, metadata, created_at, modified_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?);`,
		&sqlitex.ExecOptions{
			Args: []any{id, title, caption, filename, encMetadata, createdAt, createdAt},
		}); err != nil {
		return nil, err
	}

	return &Upload{
		ID:         id.String(),
		Title:      title,
		Caption:    caption,
		Filename:   filename,
		Metadata:   metadata,
		CreatedAt:  createdAt,
		ModifiedAt: createdAt,
	}, nil
}

func GetUploads(db *sqlitex.Pool, ctx context.Context) ([]*Upload, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	var uploads []*Upload
	fn := func(stmt *sqlite.Stmt) error {
		var metadata UploadMetadata
		err := json.Unmarshal([]byte(stmt.ColumnText(4)), &metadata)
		if err != nil {
			return err
		}

		createdAt, err := time.Parse(time.RFC3339, stmt.ColumnText(5))
		if err != nil {
			return err
		}

		modifiedAt, err := time.Parse(time.RFC3339, stmt.ColumnText(6))
		if err != nil {
			return err
		}

		uploads = append(uploads, &Upload{
			ID:         stmt.ColumnText(0),
			Title:      stmt.ColumnText(1),
			Caption:    stmt.ColumnText(2),
			Filename:   stmt.ColumnText(3),
			Metadata:   metadata,
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
		})

		return nil
	}
	if err := sqlitex.Execute(conn,
		`SELECT id, title, caption, filename, metadata, created_at, modified_at
		 FROM uploads`,
		&sqlitex.ExecOptions{
			ResultFunc: fn,
		}); err != nil {
		return nil, fmt.Errorf("unable to pull uploads: %w", err)
	}

	return uploads, nil
}
