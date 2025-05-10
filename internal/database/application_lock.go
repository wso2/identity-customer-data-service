package database

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv" // For hashing string keys to integers
)

type DistributedLock interface {
	Acquire(key string) (bool, error)
	Release(key string) error
}

// PostgresLock implements DistributedLock using PostgreSQL advisory locks.
type PostgresLock struct {
	conn *sql.Conn
}

func NewPostgresLock(conn *sql.Conn) *PostgresLock {
	return &PostgresLock{conn: conn}
}

// PostgreSQL advisory locks use bigint or two integers. We'll use a single bigint.
func (l *PostgresLock) generateLockKey(key string) (int64, error) {

	h := fnv.New64a() // FNV-1a is a good general-purpose non-cryptographic hash
	_, err := h.Write([]byte(key))
	if err != nil {
		return 0, fmt.Errorf("failed to hash lock key '%s': %w", key, err)
	}
	return int64(h.Sum64()), nil // Cast to int64 for pg_advisory_lock
}

func (l *PostgresLock) Acquire(key string) (bool, error) {
	lockID, err := l.generateLockKey(key)
	if err != nil {
		return false, err
	}

	var acquired bool
	err = l.conn.QueryRowContext(context.Background(), "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("pg_try_advisory_lock failed: %w", err)
	}
	return acquired, nil
}

func (l *PostgresLock) Release(key string) error {
	lockID, err := l.generateLockKey(key)
	if err != nil {
		return err
	}

	var released bool
	err = l.conn.QueryRowContext(context.Background(), "SELECT pg_advisory_unlock($1)", lockID).Scan(&released)
	if err != nil {
		return fmt.Errorf("pg_advisory_unlock failed: %w", err)
	}

	return nil
}
