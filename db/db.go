package db

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq" // Import postgres driver
	"log/slog"
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

const SchedulerAdvisoryLockID = 123456789012345 // Choose a unique int64 number

// TryAcquireTransactionalLock attempts to acquire a transaction-level advisory lock.
// The lock is automatically released at the end of the transaction.
func TryAcquireTransactionalLock(ctx context.Context, tx *sql.Tx, lockID int64, logger *slog.Logger) (bool, error) {
	var acquired bool
	// pg_try_advisory_xact_lock(key bigint) returns true if the lock is acquired, false otherwise.
	err := tx.QueryRowContext(ctx, "SELECT pg_try_advisory_xact_lock($1)", lockID).Scan(&acquired)
	if err != nil {
		if logger != nil {
			logger.ErrorContext(ctx, "Error executing pg_try_advisory_xact_lock", slog.Int64("lock_id", lockID), slog.Any("error", err))
		}
		return false, fmt.Errorf("failed to execute pg_try_advisory_xact_lock for lock ID %d: %w", lockID, err)
	}

	if logger != nil {
		if acquired {
			logger.InfoContext(ctx, "Successfully acquired transactional advisory lock", slog.Int64("lock_id", lockID))
		} else {
			logger.InfoContext(ctx, "Transactional advisory lock already held by another transaction or could not be acquired", slog.Int64("lock_id", lockID))
		}
	}
	return acquired, nil
}
