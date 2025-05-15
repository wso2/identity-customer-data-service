package database

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"hash/fnv" // For hashing string keys to integers
)

type DistributedLock interface {
	Acquire(key string) (bool, error)
	Release(key string) error
}

// PostgresLock implements DistributedLock using PostgreSQL advisory locks.
type PostgresLock struct{}

func NewPostgresLock() *PostgresLock {
	return &PostgresLock{}
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

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()
	lockID, err := l.generateLockKey(key)
	if err != nil {
		return false, err
	}

	var acquired bool
	results, err := dbClient.ExecuteQuery("SELECT pg_try_advisory_lock($1)", lockID)
	row := results[0]
	acquired = row["pg_try_advisory_lock"].(bool)
	if err != nil {
		return false, fmt.Errorf("pg_try_advisory_lock failed: %w", err)
	}
	return acquired, nil
}

func (l *PostgresLock) Release(key string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()
	lockID, err := l.generateLockKey(key)
	if err != nil {
		return err
	}

	var released bool
	results, err := dbClient.ExecuteQuery("SELECT pg_advisory_unlock($1)", lockID)
	row := results[0]
	released = row["pg_advisory_unlock"].(bool)
	if err != nil || !released {
		return fmt.Errorf("pg_advisory_unlock failed: %w", err)
	}

	return nil
}
