SRC = $(shell find . -iname '*.go')

photos: $(SRC) photos.db
	go build

# on schema changes, make clean and recreate the db
# TODO: when I'm reasonably stable, stop clearing the database on every build
photos.db: schema.sql
	$(MAKE) clean
	sqlite3 photos.db < schema.sql
	./ops/create_db.sh photos.db

.PHONY: clean
clean:
	rm -f uploads/*
	# copy photos.db into /tmp to provide a tiny bit of security from an
	# accidental deletion... for now I don't care too much
	[ -f photos.db ] && mv photos.db /tmp/photos.db || true
	rm -f photos.db*
