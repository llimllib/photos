CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password TEXT NOT NULL,
  created_at TEXT NOT NULL, -- datetime
  modified_at TEXT NOT NULL -- datetime
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  data JSON NOT NULL, -- { Username string }
  created_at TEXT NOT NULL -- datetime
);

CREATE TABLE IF NOT EXISTS uploads (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  caption TEXT,
  filename TEXT NOT NULL,
  content_type TEXT NOT NULL,
  metadata JSON, -- { Iso string; Camera string; Aperture string; Exposure string; Exif []exif.ExifTag; ExifMisc *exif.MiscellaneousExifData }
  created_at TEXT NOT NULL, -- datetime
  modified_at TEXT NOT NULL -- datetime
);
