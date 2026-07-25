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

package client

import (
	"database/sql"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	_ "modernc.org/sqlite"
)

// DBClientInterface defines the interface for database operations.
type DBClientInterface interface {
	ExecuteQuery(query string, args ...interface{}) ([]map[string]interface{}, error)
	BeginTx() (*sql.Tx, error)
	Close() error
}

// DBClient is the implementation of DBClientInterface.
type DBClient struct {
	db *sql.DB
	// dbType is the datasource type this client is connected to. It selects the
	// result normalization applied to scanned rows.
	dbType string
	// shared marks a client over a connection pool owned by the provider rather
	// than by this client, in which case Close must not close the pool.
	shared bool
}

// NewDBClient creates a new instance of DBClient with the provided database connection.
func NewDBClient(db *sql.DB, dbType string) DBClientInterface {

	return &DBClient{
		db:     db,
		dbType: dbType,
	}
}

// NewSharedDBClient creates a client over a connection pool owned by the caller.
// Close is a no-op, so the pool outlives the client and the existing
// `defer dbClient.Close()` calls in the stores stay correct.
func NewSharedDBClient(db *sql.DB, dbType string) DBClientInterface {

	return &DBClient{
		db:     db,
		dbType: dbType,
		shared: true,
	}
}

// ExecuteQuery executes a SELECT query and returns the result as a slice of maps.
func (client *DBClient) ExecuteQuery(query string, args ...interface{}) ([]map[string]interface{}, error) {

	isSQLite := client.dbType == database.TypeSQLite
	if isSQLite {
		args = normalizeSQLiteArgs(args)
	}

	rows, err := client.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Declared column types drive the SQLite result normalization.
	var declaredTypes []string
	if isSQLite {
		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			return nil, err
		}
		declaredTypes = make([]string, len(columnTypes))
		for i, columnType := range columnTypes {
			declaredTypes[i] = columnType.DatabaseTypeName()
		}
	}

	var results []map[string]interface{}
	for rows.Next() {
		row := make([]interface{}, len(columns))
		rowPointers := make([]interface{}, len(columns))
		for i := range row {
			rowPointers[i] = &row[i]
		}

		if err := rows.Scan(rowPointers...); err != nil {
			return nil, err
		}

		result := map[string]interface{}{}
		for i, col := range columns {
			value := row[i]
			if isSQLite {
				value = normalizeSQLiteValue(value, declaredTypes[i])
			}
			// Normalize column names to lowercase for consistency.
			result[strings.ToLower(col)] = value
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// BeginTx starts a new database transaction.
func (client *DBClient) BeginTx() (*sql.Tx, error) {

	return client.db.Begin()
}

// Close closes the database connection. It is a no-op when the connection pool
// is owned by the provider (the inbuilt database) or when running under tests.
func (c *DBClient) Close() error {
	if c.shared || os.Getenv("TEST_MODE") == "true" {
		return nil
	}
	return c.db.Close()
}
