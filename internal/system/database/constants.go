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

// Package database holds constants shared by the database provider, client and
// query layers.
package database

// Supported datasource types. These values are used both as the
// `datasource.type` configuration value and as the dialect key when looking up
// a statement in the query maps defined in the scripts package.
const (
	// TypePostgres is the PostgreSQL datasource. This is the default and the
	// recommended type for production deployments.
	TypePostgres = "postgres"
	// TypeSQLite is the inbuilt, file-backed SQLite datasource. It requires no
	// external database server and is intended for development, demos and
	// single-instance deployments.
	TypeSQLite = "sqlite"
)

// Driver names registered with database/sql by the imported driver packages:
// github.com/lib/pq registers "postgres" and modernc.org/sqlite registers
// "sqlite".
const (
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

// SQLite defaults, applied when the corresponding configuration values are
// left empty.
const (
	// DefaultSQLitePath is the inbuilt database location, resolved relative to
	// CDS_HOME when it is not an absolute path.
	DefaultSQLitePath = "repository/database/cds.db"

	// DefaultSQLiteOptions is the DSN query string appended to the database
	// file path.
	//
	// Each option is load-bearing:
	//   - foreign_keys(1)    enforces the ON DELETE CASCADE constraints in the
	//                        schema. Set through the DSN rather than a one-off
	//                        `PRAGMA foreign_keys = ON` statement so that it
	//                        applies to every pooled connection, not just the
	//                        one that happened to serve the statement.
	//   - journal_mode(WAL)  allows concurrent readers alongside a writer.
	//   - busy_timeout(5000) waits instead of failing immediately when the
	//                        write lock is held.
	//   - _txlock=immediate  takes the write lock when a transaction begins, so
	//                        a read-then-write transaction honours the busy
	//                        timeout instead of failing to upgrade its lock.
	//   - _time_format=sqlite and _timezone=UTC store time.Time values as
	//                        fixed-shape UTC text ("2006-01-02 15:04:05.999-07:00")
	//                        rather than time.Time.String(). This is mandatory:
	//                        the default format is neither sortable nor
	//                        parseable, and keyset pagination orders on these
	//                        columns.
	//   - _texttotime=true   scans TIMESTAMP/DATETIME columns back into
	//                        time.Time. Stated explicitly even though it is the
	//                        driver default.
	DefaultSQLiteOptions = "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" +
		"&_txlock=immediate&_time_format=sqlite&_timezone=UTC&_texttotime=true"

	// DefaultSQLiteMaxOpenConns bounds the connection pool. SQLite serialises
	// writers, so a small pool avoids lock contention between the HTTP handlers
	// and the background workers. WAL still permits concurrent reads.
	DefaultSQLiteMaxOpenConns = 4
)
