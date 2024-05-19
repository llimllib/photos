package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"crawshaw.io/sqlite/sqlitex"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	dbFile    = "photos.db"
	tableName = "users"
	saltStr   = "0f6f68577436a6b466b36b59504191b6"
)

func main() {
	db, err := sqlitex.Open(dbFile, 0, 10)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	conn := db.Get(context.Background())
	if conn == nil {
		panic("unable to open db connection")
	}
	defer db.Put(conn)

	passwordBytes := make([]byte, 8)
	_, err = rand.Read(passwordBytes)
	if err != nil {
		log.Fatalf("Error generating random password: %v", err)
	}
	password := hex.EncodeToString(passwordBytes)
	fmt.Printf("Password: %s\n", password)

	// Combine the password and salt string, and hash them using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password+saltStr), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}

	// Generate a UUIDv7 GUID for the id
	id, err := uuid.NewUUID()
	if err != nil {
		log.Fatalf("Error generating UUID: %v", err)
	}

	// Insert the row into the "users" table
	stmt := conn.Prep(`
		INSERT INTO users (id, username, password)
		VALUES ($id, $username, $password)
	`)
	stmt.SetText("$id", id.String())
	stmt.SetText("$username", "llimllib")
	stmt.SetBytes("$password", hashedPassword)
	if _, err := stmt.Step(); err != nil {
		panic(err.Error())
	}

	log.Println("Row inserted successfully")
}
