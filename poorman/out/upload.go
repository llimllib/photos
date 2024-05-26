package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dsoprea/go-exif/v2"
	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/google/uuid"
)

type Upload_Metadata struct {
	Iso      string
	Camera   string
	Aperture string
	Exposure string
	Exif     []exif.ExifTag
	ExifMisc *exif.MiscellaneousExifData
}

type Upload struct {
	Id          string
	Title       string
	Caption     string
	Filename    string
	Contenttype string
	Metadata    Upload_Metadata
	Createdat   time.Time
	Modifiedat  time.Time
}

func NewUpload(db *sqlitex.Pool, data *Upload, ctx context.Context) (*Upload, error) {
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
