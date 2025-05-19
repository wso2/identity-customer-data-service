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

package setup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestDatabase contains the running container and DB connection
type TestDatabase struct {
	Container *postgres.PostgresContainer
	DB        *sql.DB
}

// SetupTestDB spins up a Postgres container and applies schema.sql
func SetupTestDB(ctx context.Context) (*TestDatabase, error) {
	container, err := postgres.RunContainer(ctx,
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Load and execute schema.sql
	schemaFile := filepath.Join("test", "integration", "setup", "schema.sql")
	schemaBytes, err := os.ReadFile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema.sql: %w", err)
	}

	_, err = db.Exec(string(schemaBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to execute schema: %w", err)
	}

	return &TestDatabase{
		Container: container,
		DB:        db,
	}, nil
}
