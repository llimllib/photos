photos: *.go photos.db
	go build

photos.db: schema.sql
	./ops/create_db.sh photos.db
