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
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/database/model"
	_ "modernc.org/sqlite"
)

// DBClientInterface defines the interface for database operations.
//
// Statements are passed as a model.DBQuery rather than as text: the client owns
// the datasource type, so it is the one place that decides which dialect a
// statement runs as. Stores name a statement and stay dialect-agnostic.
type DBClientInterface interface {
	ExecuteQuery(query model.DBQuery, args ...interface{}) ([]map[string]interface{}, error)
	BeginTx() (*model.Tx, error)
	// DBType exposes the dialect for the few runtime statement builders whose
	// bind arguments — not just their SQL text — differ per engine. Stores must
	// not use it to look up a statement.
	DBType() string
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

// ExecuteQuery executes a query and returns the result as a slice of maps.
func (client *DBClient) ExecuteQuery(query model.DBQuery, args ...interface{}) (
	[]map[string]interface{}, error) {

	isSQLite := client.dbType == database.TypeSQLite
	if isSQLite {
		args = database.NormalizeSQLiteArgs(args)
	}

	// The single place a statement is resolved to a dialect.
	sqlText := query.GetQuery(client.dbType)

	rows, err := client.db.Query(sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s failed: %w", query.ID, err)
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

// BeginTx starts a new database transaction. The returned transaction carries
// the dialect, so statements run inside it are resolved the same way as those
// run through ExecuteQuery.
func (client *DBClient) BeginTx() (*model.Tx, error) {

	tx, err := client.db.Begin()
	if err != nil {
		return nil, err
	}
	return model.NewTx(tx, client.dbType), nil
}

// DBType returns the datasource type this client is connected to.
func (client *DBClient) DBType() string {

	return client.dbType
}

// Close closes the database connection. It is a no-op when the connection pool
// is owned by the caller rather than by this client.
func (client *DBClient) Close() error {

	if client.shared {
		return nil
	}
	return client.db.Close()
}
