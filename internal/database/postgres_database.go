package database

import (
	"database/sql"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
	"log"
	"sync"

	_ "github.com/lib/pq"
)

// PostgresDB struct holds the database connection
type PostgresDB struct {
	DB *sql.DB
}

var (
	postgresInstance *PostgresDB
	postgresOnce     sync.Once
)

// ConnectPostgres initializes a global PostgreSQL connection
func ConnectPostgres(host, port, user, password, dbname string) *PostgresDB {
	postgresOnce.Do(func() {
		// Build the connection string
		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)

		log.Println("Connecting to PostgreSQL...", connStr)
		// Open a connection to PostgreSQL
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			log.Fatalf("Failed to create PostgreSQL client: %v", err)
		}

		// Ping the database to ensure the connection is live
		if err = db.Ping(); err != nil {
			log.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}

		logger.Info(" Connected to PostgreSQL")

		// Assign global instance
		postgresInstance = &PostgresDB{
			DB: db,
		}
	})

	return postgresInstance
}

// GetPostgresInstance returns the PostgreSQL instance
func GetPostgresInstance() *PostgresDB {
	return postgresInstance
}
