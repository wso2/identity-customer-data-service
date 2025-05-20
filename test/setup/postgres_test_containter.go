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
	"log"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestPostgres struct {
	Container testcontainers.Container
	DB        *sql.DB
}

func SetupTestPostgres(ctx context.Context) (*TestPostgres, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")

	dsn := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=testdb sslmode=disable", host, port.Port())
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		container.Terminate(ctx)
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		container.Terminate(ctx)
		return nil, err
	}

	log.Printf("✅ Postgres container started at %s:%s", host, port.Port())

	return &TestPostgres{
		Container: container,
		DB:        db,
	}, nil
}
