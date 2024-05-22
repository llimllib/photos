package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	exif "github.com/dsoprea/go-exif/v3"
	sqlite "github.com/go-llsqlite/crawshaw"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/google/uuid"
)

type UploadMetadata struct {
	Iso      string
	Camera   string
	Aperture string
	Exposure string
	Exif     []exif.ExifTag
	ExifMisc *exif.MiscellaneousExifData
}

type Upload struct {
	ID          string
	Title       string
	Caption     string
	Filename    string
	ContentType string
	Metadata    UploadMetadata
	CreatedAt   time.Time
	ModifiedAt  time.Time
}

func NewUpload(
	db *sqlitex.Pool,
	title, caption, filename, contentType string,
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
		`INSERT INTO uploads (id, title, caption, filename, content_type, metadata, created_at, modified_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?);`,
		&sqlitex.ExecOptions{
			Args: []any{id, title, caption, filename, contentType, encMetadata, createdAt.Format(time.RFC3339), createdAt.Format(time.RFC3339)},
		}); err != nil {
		return nil, err
	}

	return &Upload{
		ID:          id.String(),
		Title:       title,
		Caption:     caption,
		Filename:    filename,
		ContentType: contentType,
		Metadata:    metadata,
		CreatedAt:   createdAt,
		ModifiedAt:  createdAt,
	}, nil
}

func ProcessImage(db *sqlitex.Pool, uploadID string, logger *slog.Logger) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic handled", "error:", r, "stacktrace", string(debug.Stack()))
		}
	}()

	upload, err := GetUploadById(db, uploadID, context.Background())
	if err != nil {
		logger.Error("Unable to get upload", "id", uploadID, "error", err)
		return
	}

	f, err := os.Open(fmt.Sprintf("uploads/%s", upload.Filename))
	if err != nil {
		logger.Error("Unable to open image", "filename", upload.Filename, "id", upload.ID)
		return
	}

	rawExif, err := exif.SearchAndExtractExifWithReader(f)
	if err != nil {
		if errors.Is(err, exif.ErrNoExif) {
			logger.Debug("no exif found", "filename", upload.Filename, "id", upload.ID, "error", err.Error())
			return
		}
		logger.Error("error reading possible exif data", "filename", upload.Filename, "id", upload.ID, "error", err.Error())
		return
	}

	tags, misc, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil {
		logger.Error("error parsing exif data", "filename", upload.Filename, "id", upload.ID, "error", err.Error())
		return
	}

	upload.Metadata.Exif = tags
	upload.Metadata.ExifMisc = misc

	logger.Debug("processed metadata:", "metadata", upload.Metadata)

	UpdateUpload(db, upload, context.Background())
}

func UpdateUpload(db *sqlitex.Pool, upload *Upload, ctx context.Context) error {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	encMetadata, err := json.Marshal(upload.Metadata)
	if err != nil {
		return err
	}

	if err := sqlitex.Execute(conn,
		`UPDATE uploads
		SET title=?, caption=?, filename=?, content_type=?, metadata=?, created_at=?, modified_at=?
		WHERE id=?`,
		&sqlitex.ExecOptions{
			Args: []any{
				upload.Title,
				upload.Caption,
				upload.Filename,
				upload.ContentType,
				encMetadata,
				upload.CreatedAt.Format(time.RFC3339),
				upload.ModifiedAt.Format(time.RFC3339),
				upload.ID,
			},
		}); err != nil {
		return err
	}
	return nil
}

func GetUploadById(db *sqlitex.Pool, id string, ctx context.Context) (*Upload, error) {
	conn := db.Get(ctx)
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	var upload *Upload
	fn := func(stmt *sqlite.Stmt) error {
		var metadata UploadMetadata
		err := json.Unmarshal([]byte(stmt.ColumnText(5)), &metadata)
		if err != nil {
			return err
		}

		createdAt, err := time.Parse(time.RFC3339, stmt.ColumnText(6))
		if err != nil {
			return err
		}

		modifiedAt, err := time.Parse(time.RFC3339, stmt.ColumnText(7))
		if err != nil {
			return err
		}

		upload = &Upload{
			ID:          stmt.ColumnText(0),
			Title:       stmt.ColumnText(1),
			Caption:     stmt.ColumnText(2),
			Filename:    stmt.ColumnText(3),
			ContentType: stmt.ColumnText(4),
			Metadata:    metadata,
			CreatedAt:   createdAt,
			ModifiedAt:  modifiedAt,
		}

		return nil
	}
	if err := sqlitex.Execute(conn,
		`SELECT id, title, caption, filename, content_type, metadata, created_at, modified_at
		 FROM uploads
		 WHERE id=?`,
		&sqlitex.ExecOptions{
			ResultFunc: fn,
			Args:       []any{id},
		}); err != nil {
		return nil, fmt.Errorf("unable to pull uploads: %w", err)
	}

	return upload, nil
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
