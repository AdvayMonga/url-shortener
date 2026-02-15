package main

import (
	//"net/http"
	"database/sql"
	"fmt"
	"log"
	// "math/big"
	// "encoding/json"
	// "time"
	// "crypto/rand"
	// "strings"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func createTable() {
	query := 
	"CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		short_code TEXT UNIQUE NOT NULL,
		original_url TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		click_count INTEGER DEFAULT 0
	)"

	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error creating table:", err)
	}
	fmt.Println("Table ready")
}

func main() {
	fmt.Println("Starting URL shortener...")

	var err error
	db, err = sql.Open("sqlite3", "./url_shortener.db")
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	defer db.Close()

	fmt.Println("Database file created")

	createTable()
}

