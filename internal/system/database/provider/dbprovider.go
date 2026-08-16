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
	"path/filepath"
	"strings"
	"sync"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/database/client"
)

// DBConfig represents the local database configuration.
type DBConfig struct {
	dsn        string
	driverName string
}

var (
	testDBOverride     *sql.DB
	testDBTypeOverride string
)

// SetTestDB installs a database handle used by every subsequent GetDBClient
// call, bypassing the configured datasource. dbType selects the SQL dialect and
// the scan normalization behaviour; an empty value defaults to PostgreSQL so
// existing callers keep their current semantics.
func SetTestDB(db *sql.DB, dbType string) {
	testDBOverride = db
	testDBTypeOverride = dbType
}

// sqliteHandle caches the single *sql.DB used for the inbuilt database.
//
// Unlike PostgreSQL, where each store call opens and closes its own connection,
// SQLite is a local file: reopening it per call would re-acquire locks and
// checkpoint the WAL on every query. One shared pooled handle is opened on first
// use instead, and the client handed to callers treats Close as a no-op so the
// existing `defer dbClient.Close()` calls in the stores remain correct.
var (
	sqliteHandle *sql.DB
	sqliteOnce   sync.Once
	sqliteErr    error
)

// DBProviderInterface defines the interface for getting database clients.
type DBProviderInterface interface {
	GetDBClient() (client.DBClientInterface, error)
	GetDBType() string
}

// DBProvider is the implementation of DBProviderInterface.
type DBProvider struct{}

// NewDBProvider creates a new instance of DBProvider.
func NewDBProvider() DBProviderInterface {

	return &DBProvider{}
}

// GetDBClient returns a database client for the configured datasource.
func (d *DBProvider) GetDBClient() (client.DBClientInterface, error) {

	// The test handle is owned by the suite that installed it and outlives every
	// client handed out from it, so Close must not close the pool. A shared
	// client makes that true by construction, rather than depending on an
	// environment variable being set.
	if testDBOverride != nil {
		return client.NewSharedDBClient(testDBOverride, resolveDBType(testDBTypeOverride)), nil
	}

	// Production DB setup
	runtimeConfig := config.GetCDSRuntime().Config
	dbType := resolveDBType(runtimeConfig.DataSource.Type)

	// The inbuilt database is backed by a single shared handle.
	if dbType == database.TypeSQLite {
		db, err := getSQLiteDB()
		if err != nil {
			return nil, err
		}
		return client.NewSharedDBClient(db, dbType), nil
	}

	dbConfig, err := getDBConfig(runtimeConfig)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dbConfig.driverName, dbConfig.dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Test the database connection.
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return client.NewDBClient(db, dbType), nil
}

// getSQLiteDB opens the inbuilt database once and initializes its schema.
func getSQLiteDB() (*sql.DB, error) {

	sqliteOnce.Do(func() {
		runtimeConfig := config.GetCDSRuntime()

		dbConfig, err := getDBConfig(runtimeConfig.Config)
		if err != nil {
			sqliteErr = err
			return
		}

		if err := ensureSQLiteDir(runtimeConfig.Config.DataSource.SQLite.Path); err != nil {
			sqliteErr = err
			return
		}

		db, err := sql.Open(dbConfig.driverName, dbConfig.dsn)
		if err != nil {
			sqliteErr = fmt.Errorf("failed to open the inbuilt database: %v", err)
			return
		}

		maxOpenConns := runtimeConfig.Config.DataSource.SQLite.MaxOpenConns
		if maxOpenConns <= 0 {
			maxOpenConns = database.DefaultSQLiteMaxOpenConns
		}
		db.SetMaxOpenConns(maxOpenConns)
		db.SetMaxIdleConns(maxOpenConns)

		if err := db.Ping(); err != nil {
			_ = db.Close()
			sqliteErr = fmt.Errorf("failed to ping the inbuilt database: %v", err)
			return
		}

		if err := initializeSQLiteSchema(db); err != nil {
			_ = db.Close()
			sqliteErr = err
			return
		}

		sqliteHandle = db
	})

	return sqliteHandle, sqliteErr
}

// getDBConfig returns the database configuration based on the provided data source.
func getDBConfig(dataSource config.Config) (DBConfig, error) {

	ds := dataSource.DataSource

	switch resolveDBType(ds.Type) {
	case database.TypeSQLite:
		path, err := resolveSQLitePath(ds.SQLite.Path)
		if err != nil {
			return DBConfig{}, err
		}

		options := ds.SQLite.Options
		if options == "" {
			options = database.DefaultSQLiteOptions
		}
		if !strings.HasPrefix(options, "?") {
			options = "?" + options
		}

		return DBConfig{
			driverName: database.DriverSQLite,
			dsn:        path + options,
		}, nil

	default:
		// PostgreSQL. The driver name is taken verbatim from the configured type
		// so that any other database/sql driver registered under that name keeps
		// working exactly as before.
		driverName := ds.Type
		if driverName == "" {
			driverName = database.DriverPostgres
		}

		return DBConfig{
			driverName: driverName,
			dsn: fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
				ds.Hostname, ds.Port, ds.Username, ds.Password, ds.Name, ds.SSLMode),
		}, nil
	}
}

// resolveSQLitePath returns the absolute path of the inbuilt database file,
// resolving a relative path against CDS_HOME.
func resolveSQLitePath(path string) (string, error) {

	if path == "" {
		path = database.DefaultSQLitePath
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(config.GetCDSRuntime().CDSHome, path), nil
}

// resolveDBType normalizes a configured datasource type. An empty value means
// PostgreSQL, preserving the behaviour of deployments that never set it.
func resolveDBType(dbType string) string {

	normalized := strings.ToLower(strings.TrimSpace(dbType))
	if normalized == "" {
		return database.TypePostgres
	}
	return normalized
}

// GetDBType returns the datasource type, which is also the dialect key used to
// select a statement from the query maps in the scripts package.
func (d *DBProvider) GetDBType() string {

	if testDBOverride != nil {
		return resolveDBType(testDBTypeOverride)
	}
	return resolveDBType(config.GetCDSRuntime().Config.DataSource.Type)
}
