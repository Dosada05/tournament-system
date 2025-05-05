package db

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq" // Import postgres driver
	"time"
)

func Connect(dsn string, timeout time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create database handle: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Printf("failed to close database handle after ping error: %v\n", closeErr)
		}
		return nil, fmt.Errorf("failed to ping database within %v: %w", timeout, err)
	}

	return db, nil
}
