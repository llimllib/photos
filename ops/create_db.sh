#!/bin/bash

db_path="$1"
schema_path="schema.sql"

mv "$db_path" /tmp/photos.db.bak
sqlite3 "$db_path" < "$schema_path"
go run ops/create_admin.go
