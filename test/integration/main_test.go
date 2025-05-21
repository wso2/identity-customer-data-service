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

package integration

import (
	"context"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"github.com/wso2/identity-customer-data-service/test/integration/utils"
	"github.com/wso2/identity-customer-data-service/test/setup"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	os.Setenv("TEST_MODE", "true") // ✅ Add this

	conf := config.Config{
		Log: config.LogConfig{
			LogLevel: "DEBUG",
		},
	}
	config.OverrideCDSRuntime(conf)
	_ = log.Init("DEBUG")

	pg, err := setup.SetupTestPostgres(ctx)
	if err != nil {
		fmt.Println("Failed to start test DB:", err)
		os.Exit(1)
	}

	workers.StartProfileWorker() // Start the real enrichment queue worker

	provider.SetTestDB(pg.DB)
	err = utils.CreateTablesFromFile(pg.DB, "/Users/admin/Documents/Repos/OnPrem/identity-customer-data-service/test/setup/schema.sql")
	if err != nil {
		fmt.Println("Failed to create tables from schema:", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Terminate container manually after tests complete
	_ = pg.Container.Terminate(ctx)

	os.Exit(code)
}
