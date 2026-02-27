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

package benchmark

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"github.com/wso2/identity-customer-data-service/test/integration/utils"
	"github.com/wso2/identity-customer-data-service/test/setup"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	os.Setenv("TEST_MODE", "true")

	conf := config.Config{
		Log: config.LogConfig{
			LogLevel: "ERROR", // Use ERROR for benchmarks to reduce noise
		},
		DataSource: config.DataSourceConfig{
			Type: "postgres",
		},
	}
	config.OverrideCDSRuntime(conf)
	_ = log.Init("ERROR")

	pg, err := setup.SetupTestPostgres(ctx)
	if err != nil {
		fmt.Println("Failed to start test DB:", err)
		os.Exit(1)
	}

	workers.StartProfileWorker() // Start the real enrichment queue worker

	// Initialize Schema Sync worker
	workers.StartSchemaSyncWorker()

	provider.SetTestDB(pg.DB)
	err = utils.CreateTablesFromFile(pg.DB, utils.GetSchemaPath())
	if err != nil {
		fmt.Println("Failed to create tables from schema:", err)
		os.Exit(1)
	}

	// Run benchmarks
	code := m.Run()

	// Terminate container manually after benchmarks complete
	_ = pg.Container.Terminate(ctx)

	// Remove the docker image used for tests
	cmd := exec.Command("docker", "rm", "-f", "cds-test-postgres")
	_, _ = cmd.CombinedOutput()

	os.Exit(code)
}
