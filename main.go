package main

import (
	"net"
	"net/http"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"log"
	"time"
	"math/rand"
	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
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
		expires_at DATETIME NOT NULL,
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
		// Serve the HTML interface
		http.ServeFile(w, r, "index.html")
		return
	}

	var originalURL string
	var expiresAt time.Time

	query := `SELECT original_url, expires_at FROM urls WHERE short_code = ?`
	err := db.QueryRow(query, shortCode).Scan(&originalURL, &expiresAt)

	if err != nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	if time.Now().After(expiresAt) {
		deleteQuery := `DELETE FROM urls WHERE short_code = ?`
		db.Exec(deleteQuery, shortCode)
		
		http.Error(w, "This short URL has expired", http.StatusGone)
		fmt.Printf("Deleted expired URL: %s\n", shortCode)
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
	expiresAt := time.Now().AddDate(0,0,7)

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
		query := `INSERT INTO urls (short_code, original_url, expires_at) VALUES (?, ?, ?)`
		_, err = db.Exec(query, shortCode, request.URL, expiresAt)
		if err != nil {
			http.Error(w, "Error creating short URL", http.StatusInternalServerError)
			return
		}

	} else {
		shortCode = generateShortCode()

		for codeExists(shortCode) {
			shortCode = generateShortCode()
		}

		query := `INSERT INTO urls (short_code, original_url, expires_at) VALUES (?, ?, ?)`
		_, err := db.Exec(query, shortCode, request.URL, expiresAt)
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

func statsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract short code from URL path
	// URL will be like: /stats/abc123
	shortCode := r.URL.Path[len("/stats/"):]
	
	if shortCode == "" {
		http.Error(w, "Short code required", http.StatusBadRequest)
		return
	}
	
	// Query database for stats
	var originalURL string
	var createdAt time.Time
	var expiresAt time.Time
	var clickCount int
	
	query := `SELECT original_url, created_at, expires_at, click_count FROM urls WHERE short_code = ?`
	err := db.QueryRow(query, shortCode).Scan(&originalURL, &createdAt, &expiresAt, &clickCount)
	
	if err != nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}
	
	// Calculate if expired
	isExpired := time.Now().After(expiresAt)
	
	// Build response
	stats := struct {
		ShortCode   string `json:"short_code"`
		OriginalURL string `json:"original_url"`
		CreatedAt   string `json:"created_at"`
		ExpiresAt   string `json:"expires_at"`
		ClickCount  int    `json:"click_count"`
		IsExpired   bool   `json:"is_expired"`
	}{
		ShortCode:   shortCode,
		OriginalURL: originalURL,
		CreatedAt:   createdAt.Format("2006-01-02 15:04:05"),
		ExpiresAt:   expiresAt.Format("2006-01-02 15:04:05"),
		ClickCount:  clickCount,
		IsExpired:   isExpired,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
	
	fmt.Printf("Stats requested for: %s\n", shortCode)
}

type visitor struct {
	count    int
	lastSeen time.Time
}

var visitors = make(map[string]*visitor)
var mu sync.Mutex

func rateLimiter(next http.HandlerFunc) http.HandlerFunc {
	const maxRequests = 100
	const window = 1 * time.Minute

	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		mu.Lock()
		v, exists := visitors[ip]

		if !exists {
			visitors[ip] = &visitor{count: 1, lastSeen: time.Now()}
			mu.Unlock()
			next(w, r)
			return
		}

		if time.Since(v.lastSeen) > window {
			v.count = 1
			v.lastSeen = time.Now()
			mu.Unlock()
			next(w, r)
			return
		}

		if v.count >= maxRequests {
			mu.Unlock()
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		v.count++
		mu.Unlock()
		next(w, r)
	}
}

func qrHandler(w http.ResponseWriter, r *http.Request) {
	shortCode := r.URL.Path[len("/qr/"):]

	if shortCode == "" {
		http.Error(w, "Short code required", http.StatusBadRequest)
		return
	}

	// Verify the short code exists
	if !codeExists(shortCode) {
		http.Error(w, "Short code not found", http.StatusNotFound)
		return
	}

	shortURL := "http://localhost:8080/" + shortCode

	png, err := qrcode.Encode(shortURL, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Error generating QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(png)

	fmt.Printf("QR code generated for: %s\n", shortCode)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	err := db.Ping()

	status := "ok"
	dbStatus := "connected"
	httpStatus := http.StatusOK

	if err != nil {
		status = "unhealthy"
		dbStatus = "disconnected"
		httpStatus = http.StatusInternalServerError
	}

	response := struct {
		Status   string `json:"status"`
		DBStatus string `json:"database"`
	}{
		Status:   status,
		DBStatus: dbStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(response)
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
    http.HandleFunc("/stats/", statsHandler)
	http.HandleFunc("/qr/", qrHandler)
	http.HandleFunc("/health", healthHandler)

    
    fmt.Println("Server running on http://localhost:8080")
    fmt.Println("POST /shorten - Create short URL")
    fmt.Println("GET /{code} - Redirect to original URL")
    fmt.Println("GET /stats/{code} - Get URL stats")
	fmt.Println("GET /qr/{code} - Get QR code")
	fmt.Println("GET /health - Health check")

    
    log.Fatal(http.ListenAndServe(":8080", rateLimiter(http.DefaultServeMux.ServeHTTP)))
}

