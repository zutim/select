package portfolio

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/glebarez/go-sqlite" // SQLite driver
)

var DB *sql.DB

func InitDB(dataSourceName string) {
	var err error
	DB, err = sql.Open("sqlite", dataSourceName)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	// Create positions table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS positions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		stock_code TEXT NOT NULL,
		buy_price REAL NOT NULL,
		buy_time DATETIME NOT NULL,
		status TEXT NOT NULL DEFAULT 'open',
		sell_price REAL,
		sell_time DATETIME
	);`

	_, err = DB.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Error creating positions table: %v", err)
	}
	log.Println("Database initialized and 'positions' table ensured.")
}

func CloseDB() {
	if DB != nil {
		DB.Close()
		log.Println("Database connection closed.")
	}
}
