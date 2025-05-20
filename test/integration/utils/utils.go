package utils

import (
	"database/sql"
	"fmt"
	"os"
)

func CreateTablesFromFile(db *sql.DB, path string) error {
	schemaBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	_, err = db.Exec(string(schemaBytes))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}
