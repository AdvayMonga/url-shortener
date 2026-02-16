package main

import (
	"net/http"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"math/rand"
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
	db.QueryRow(query, code).Scan(&exists)
	
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

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.URL.Path[1:]

	if shortCode == "" {
		fmt.Fprintf(w, "URL Shortener is running")
		return
	}

	var originalURL string
	query := `SELECT original_url FROM urls WHERE short_code = ?`
	err := db.QueryRow(query, shortCode).Scan(&originalURL)

	if err != nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	updateQuery := `UPDATE urls SET click_count = click_count + 1 WHERE short_code = ?`
	db.Exec(updateQuery, shortCode)

	http.Redirect(w, r, originalURL, http.StatusFound)
	fmt.Printf("Redirected %s -> %s\n", shortCode, originalURL)
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusBadRequest)
		return
	}

	var request struct {
		URL 		string `json:"url"`
		CustomCode 	string `json:"custom_code"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil || request.URL == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var shortCode string

	if request.CustomCode != "" {
		if len(request.CustomCode) < 3 || len(request.CustomCode) > 20 {
			http.Error(w, "Custom code must be 3-20 characters", http.StatusBadRequest)
			return
		}

		if codeExists(request.CustomCode) {
			http.Error(w, "Custom code already taken", http.StatusConflict)
			return
		}
		
		shortCode = request.CustomCode
		query := `INSERT INTO urls (short_code, original_url) VALUES (?, ?)`
		_, err = db.Exec(query, shortCode, request.URL)
		if err != nil {
			http.Error(w, "Error creating short URL", http.StatusInternalServerError)
			return
		}

	} else {
		shortCode, err = shortenURL(request.URL)
		if err != nil {
			http.Error(w, "Error creating short URL", http.StatusInternalServerError)
			return
		}
	}

	response := struct {
		ShortCode string `json:"short_code"`
		ShortURL string `json:"short_url"`
	}{
		ShortCode: shortCode,
		ShortURL: "http://localhost:8080/" + shortCode,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	fmt.Printf("Created short URL: %s -> %s\n", shortCode, request.URL)
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

    http.HandleFunc("/shorten", shortenHandler)  
    http.HandleFunc("/", redirectHandler)       
    
    fmt.Println("Server running on http://localhost:8080")
    fmt.Println("POST /shorten - Create short URL")
    fmt.Println("GET /{code}   - Redirect to original URL")
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}

