/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package provider

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wso2/identity-customer-data-service/dbscripts"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// EnsureDatabase prepares the configured datasource for use.
//
// For the inbuilt datasource it creates the database file and applies the
// schema. For every other type it is a no-op, since the schema is applied by
// the operator. It is safe to call more than once.
func EnsureDatabase() error {

	runtimeConfig := config.GetCDSRuntime().Config
	if resolveDBType(runtimeConfig.DataSource.Type) != database.TypeSQLite {
		return nil
	}

	// Opening the handle creates the file and applies the schema.
	if _, err := getSQLiteDB(); err != nil {
		return err
	}

	path, err := resolveSQLitePath(runtimeConfig.DataSource.SQLite.Path)
	if err != nil {
		return err
	}
	log.GetLogger().Info(fmt.Sprintf("Inbuilt database initialized at %s", path))

	return nil
}

// ensureSQLiteDir creates the directory that holds the inbuilt database file.
func ensureSQLiteDir(configuredPath string) error {

	path, err := resolveSQLitePath(configuredPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create the inbuilt database directory %s: %v", dir, err)
	}

	return nil
}

// initializeSQLiteSchema applies the embedded schema. The script is idempotent,
// so applying it to an existing database is a no-op.
func initializeSQLiteSchema(db *sql.DB) error {

	// A transaction keeps a partial failure from leaving a half-built database.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin the inbuilt database schema transaction: %v", err)
	}

	if _, err := tx.Exec(dbscripts.SQLiteSchema); err != nil {
		_ = tx.Rollback()

		// Fall back to statement-by-statement execution in case a future driver
		// version stops accepting multi-statement scripts.
		if fallbackErr := applySchemaStatements(db, dbscripts.SQLiteSchema); fallbackErr != nil {
			return fmt.Errorf("failed to apply the inbuilt database schema: %v (%v)", err, fallbackErr)
		}
		return nil
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit the inbuilt database schema: %v", err)
	}

	return nil
}

// applySchemaStatements executes a DDL script one statement at a time. Comments
// are stripped first, since one may contain a semicolon and split the script in
// the wrong place.
func applySchemaStatements(db *sql.DB, script string) error {

	for _, statement := range strings.Split(stripSQLComments(script), ";") {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		if _, err := db.Exec(trimmed); err != nil {
			return fmt.Errorf("failed to execute %q: %v", truncate(trimmed, 80), err)
		}
	}

	return nil
}

// stripSQLComments removes whole-line `--` comments from a statement.
func stripSQLComments(statement string) string {

	var builder strings.Builder
	for _, line := range strings.Split(statement, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String()
}

// truncate shortens a string for error messages.
func truncate(value string, max int) string {

	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
