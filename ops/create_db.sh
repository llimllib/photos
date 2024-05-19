#!/bin/bash

db_path="$1"
schema_path="schema.sql"

sqlite3 "$db_path" < "$schema_path"
