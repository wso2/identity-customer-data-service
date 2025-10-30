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

package utils

import (
	"database/sql"
	"errors"
	"fmt"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"log"
	"os"
	"path/filepath"
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

func GetTestHome() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	for {
		testSetupPath := filepath.Join(dir, "test", "setup", "schema.sql")
		if _, err := os.Stat(testSetupPath); err == nil {
			return dir // Found project root
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatalf("Project root with schema.sql not found")
		}
		dir = parent
	}
}

// GetSchemaPath returns the full path to the schema.sql file
func GetSchemaPath() string {
	return filepath.Join(GetTestHome(), "test", "setup", "schema.sql")
}

func ExtractErrorDescription(err error) string {
	var clientErr *errors2.ClientError
	if errors.As(err, &clientErr) {
		return clientErr.Description
	}
	return err.Error()
}
