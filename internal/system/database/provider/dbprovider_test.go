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
	"path/filepath"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
)

// postgresDataSource is the configuration of an existing PostgreSQL
// deployment, with no sqlite block.
func postgresDataSource(dbType string) config.Config {

	return config.Config{
		DataSource: config.DataSourceConfig{
			Type:     dbType,
			Hostname: "localhost",
			Port:     5432,
			Username: "cdsuser",
			Password: "cdspwd",
			Name:     "cdsdb",
			SSLMode:  "disable",
		},
	}
}

// Test_getDBConfig_postgres pins the PostgreSQL DSN, which the inbuilt database
// must not change.
func Test_getDBConfig_postgres(t *testing.T) {

	expectedDSN := "host=localhost port=5432 user=cdsuser password=cdspwd dbname=cdsdb sslmode=disable"

	t.Run("configured as postgres", func(t *testing.T) {
		dbConfig, err := getDBConfig(postgresDataSource("postgres"))
		if err != nil {
			t.Fatal(err)
		}
		if dbConfig.dsn != expectedDSN {
			t.Errorf("expected %q, got %q", expectedDSN, dbConfig.dsn)
		}
		if dbConfig.driverName != "postgres" {
			t.Errorf("expected the postgres driver, got %q", dbConfig.driverName)
		}
	})

	t.Run("type not configured at all", func(t *testing.T) {
		dbConfig, err := getDBConfig(postgresDataSource(""))
		if err != nil {
			t.Fatal(err)
		}
		if dbConfig.dsn != expectedDSN {
			t.Errorf("expected %q, got %q", expectedDSN, dbConfig.dsn)
		}
		if dbConfig.driverName != database.DriverPostgres {
			t.Errorf("expected the postgres driver, got %q", dbConfig.driverName)
		}
	})

	t.Run("an unrecognized type keeps using its own driver name", func(t *testing.T) {
		dbConfig, err := getDBConfig(postgresDataSource("pgx"))
		if err != nil {
			t.Fatal(err)
		}
		if dbConfig.dsn != expectedDSN {
			t.Errorf("expected %q, got %q", expectedDSN, dbConfig.dsn)
		}
		if dbConfig.driverName != "pgx" {
			t.Errorf("expected the configured driver name, got %q", dbConfig.driverName)
		}
	})
}

func Test_getDBConfig_sqlite(t *testing.T) {

	config.OverrideCDSRuntime(config.Config{})
	home := config.GetCDSRuntime().CDSHome

	testCases := []struct {
		name        string
		sqlite      config.SQLiteConfig
		expectedDSN string
	}{
		{
			name:        "no sqlite block configured",
			sqlite:      config.SQLiteConfig{},
			expectedDSN: filepath.Join(home, database.DefaultSQLitePath) + "?" + database.DefaultSQLiteOptions,
		},
		{
			name:        "a relative path resolves against CDS_HOME",
			sqlite:      config.SQLiteConfig{Path: "data/cds.db"},
			expectedDSN: filepath.Join(home, "data/cds.db") + "?" + database.DefaultSQLiteOptions,
		},
		{
			name:        "an absolute path is used as given",
			sqlite:      config.SQLiteConfig{Path: "/var/lib/cds/cds.db"},
			expectedDSN: "/var/lib/cds/cds.db?" + database.DefaultSQLiteOptions,
		},
		{
			name:        "configured options replace the defaults",
			sqlite:      config.SQLiteConfig{Path: "/tmp/cds.db", Options: "_pragma=busy_timeout(1000)"},
			expectedDSN: "/tmp/cds.db?_pragma=busy_timeout(1000)",
		},
		{
			name:        "configured options may include the separator",
			sqlite:      config.SQLiteConfig{Path: "/tmp/cds.db", Options: "?_pragma=busy_timeout(1000)"},
			expectedDSN: "/tmp/cds.db?_pragma=busy_timeout(1000)",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dbConfig, err := getDBConfig(config.Config{
				DataSource: config.DataSourceConfig{Type: "sqlite", SQLite: testCase.sqlite},
			})
			if err != nil {
				t.Fatal(err)
			}
			if dbConfig.driverName != database.DriverSQLite {
				t.Errorf("expected the sqlite driver, got %q", dbConfig.driverName)
			}
			if dbConfig.dsn != testCase.expectedDSN {
				t.Errorf("expected %q, got %q", testCase.expectedDSN, dbConfig.dsn)
			}
		})
	}
}

func Test_resolveDBType(t *testing.T) {

	testCases := map[string]string{
		"":          database.TypePostgres,
		"   ":       database.TypePostgres,
		"postgres":  database.TypePostgres,
		"Postgres":  database.TypePostgres,
		"sqlite":    database.TypeSQLite,
		" SQLite  ": database.TypeSQLite,
	}

	for configured, expected := range testCases {
		if actual := resolveDBType(configured); actual != expected {
			t.Errorf("resolveDBType(%q) = %q, expected %q", configured, actual, expected)
		}
	}
}

// Test_unsupportedDBTypeIsRejected records that normalization is not
// validation: resolveDBType passes an unrecognised value through, and the
// startup check is what refuses to run on it.
func Test_unsupportedDBTypeIsRejected(t *testing.T) {

	if normalized := resolveDBType(" MySQL "); normalized != "mysql" {
		t.Errorf("resolveDBType(%q) = %q, expected %q", " MySQL ", normalized, "mysql")
	}
	if database.IsSupportedType("mysql") {
		t.Error("mysql is not a datasource CDS can run on and must not be reported as supported")
	}
	for _, supported := range []string{database.TypePostgres, database.TypeSQLite} {
		if !database.IsSupportedType(supported) {
			t.Errorf("%q must be reported as supported", supported)
		}
	}
}

func Test_ensureSQLiteDir(t *testing.T) {

	config.OverrideCDSRuntime(config.Config{})

	path := filepath.Join(t.TempDir(), "nested", "deeper", "cds.db")
	if err := ensureSQLiteDir(path); err != nil {
		t.Fatal(err)
	}

	// Creating a directory that already exists must not fail either.
	if err := ensureSQLiteDir(path); err != nil {
		t.Fatalf("expected a second call to be a no-op: %v", err)
	}
}
