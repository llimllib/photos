SRC = $(shell find . -iname '*.go')

photos: $(SRC) photos.db
	go build

# on schema changes, clear the uploads dir, drop the database, and re-create it
# with an admin user
photos.db: schema.sql
	rm uploads/*
	./ops/create_db.sh photos.db
