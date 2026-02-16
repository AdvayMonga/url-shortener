package main

import (
	"net/http"
	"database/sql"
	"fmt"
	"log"
	// "math/big"
	// "encoding/json"
	"time"
	"math/rand"
	// "strings"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func generateShortCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const codeLength = 6

	rand.Seed(time.Now().UnixNano())

	code := make([]byte, codeLength)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}

	return string(code)
}

func shortenURL(originalURL string) (string, error) {
	shortCode := generateShortCode()

	for codeExists(shortCode) {
		shortCode = generateShortCode()
	}

	query := `INSERT INTO urls (short_code, original_url) VALUES (?, ?)`
	_, err := db.Exec(query, shortCode, originalURL)
	if err != nil {
		return "", err
	}

	return shortCode, nil
}

func codeExists(code string) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = ?)`
	db.QueryRow(query).Scan(&exists)
	
	return exists
}

func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		short_code TEXT UNIQUE NOT NULL,
		original_url TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		click_count INTEGER DEFAULT 0
	)`

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

	shortCode, err := shortenURL("https://qlab.umd.edu/people/thomas-barthel")
	if err != nil {
		log.Fatal("Error shortening URL:", err)
	}


}

