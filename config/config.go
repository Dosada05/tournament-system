package config

import (
	"database/sql"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"log"
	"os"
)

var (
	DB *sql.DB
)

func LoadConfig() {
	var err error

	err = godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	DB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

}
