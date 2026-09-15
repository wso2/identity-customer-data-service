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
	"sync"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database/client"
)

// Default pool sizing, used when deployment.yaml does not specify any.
const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 5 * time.Minute
)

// The pool is opened once and shared. database/sql is already a pool: every caller asking
// for a client gets a handle onto the same set of connections, and connections are returned
// to it rather than torn down.
//
// Each call used to sql.Open a fresh pool, Ping it, and Close it again, which made every
// single query a TCP, TLS and authentication handshake. One profile write does dozens of
// queries and the resolution pipeline does dozens more per candidate, so a single profile
// update could open hundreds of connections in series — and any configured pool limit was
// meaningless because no pool outlived one statement.
var (
	poolOnce sync.Once
	pool     *sql.DB
	poolErr  error
)

// DBConfig represents the local database configuration.
type DBConfig struct {
	dsn        string
	driverName string
}

var testDBOverride *sql.DB

func SetTestDB(db *sql.DB) {
	testDBOverride = db
}

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

// GetDBClient returns a client onto the shared connection pool, opening it on first use.
func (d *DBProvider) GetDBClient() (client.DBClientInterface, error) {

	if testDBOverride != nil {
		return client.NewDBClient(testDBOverride), nil
	}

	poolOnce.Do(func() {
		runtimeConfig := config.GetCDSRuntime().Config
		dbConfig := getDBConfig(runtimeConfig)

		db, err := sql.Open(dbConfig.driverName, dbConfig.dsn)
		if err != nil {
			poolErr = fmt.Errorf("failed to connect to database: %v", err)
			return
		}

		db.SetMaxOpenConns(orDefault(runtimeConfig.DataSource.MaxOpenConns, defaultMaxOpenConns))
		db.SetMaxIdleConns(orDefault(runtimeConfig.DataSource.MaxIdleConns, defaultMaxIdleConns))
		if seconds := runtimeConfig.DataSource.ConnMaxLifetime; seconds > 0 {
			db.SetConnMaxLifetime(time.Duration(seconds) * time.Second)
		} else {
			db.SetConnMaxLifetime(defaultConnMaxLifetime)
		}

		// Ping once, when the pool is established, rather than before every statement.
		if err := db.Ping(); err != nil {
			_ = db.Close()
			poolErr = fmt.Errorf("failed to ping database: %v", err)
			return
		}

		pool = db
	})

	if poolErr != nil {
		// A failed first attempt must not poison the process for its lifetime.
		poolOnce = sync.Once{}
		err := poolErr
		poolErr = nil
		return nil, err
	}

	return client.NewDBClient(pool), nil
}

func orDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// ClosePool shuts the shared pool down. Intended for process shutdown only.
func ClosePool() error {
	if pool == nil {
		return nil
	}
	err := pool.Close()
	pool = nil
	poolOnce = sync.Once{}
	return err
}

// getDBConfig returns the database configuration based on the provided data source.
func getDBConfig(dataSource config.Config) DBConfig {

	var dbConfig DBConfig

	dbConfig.driverName = dataSource.DataSource.Type
	dbConfig.dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dataSource.DataSource.Hostname, dataSource.DataSource.Port, dataSource.DataSource.Username, dataSource.DataSource.Password,
		dataSource.DataSource.Name, dataSource.DataSource.SSLMode)

	return dbConfig
}

// GetDBType returns the database configuration based on the provided data source.
func (d *DBProvider) GetDBType() string {

	runtimeConfig := config.GetCDSRuntime().Config
	dbConfig := getDBConfig(runtimeConfig)
	return dbConfig.driverName
}
