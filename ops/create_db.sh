#!/bin/bash

set -euxo pipefail

db_path="${1:-photos.db}"
schema_path="schema.sql"

[ -f "$db_path" ] && mv "$db_path" /tmp/photos.db.bak
sqlite3 "$db_path" < "$schema_path"
go run ops/create_admin.go
