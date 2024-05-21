CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  data JSON NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS uploads (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  caption TEXT,
	filename TEXT NOT NULL,
	metadata JSON,
	created_at TEXT NOT NULL,
	modified_at TEXT NOT NULL
);
