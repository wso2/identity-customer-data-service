/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

// Package sqlite_integration runs identity resolution against the inbuilt datasource.
//
// The PostgreSQL integration suite needs Docker, which puts it out of reach on machines
// where the daemon is unavailable and makes it the first thing skipped when it is slow. The
// inbuilt datasource needs nothing but a temp directory, so the paths that matter most on
// upgrade can be exercised on every machine and in every CI job. Where behaviour is
// dialect-sensitive the PostgreSQL suite remains the authority.
package sqlite_integration

import (
	"fmt"
	"os"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"github.com/wso2/identity-customer-data-service/test/setup"

	irWorker "github.com/wso2/identity-customer-data-service/internal/identity_resolution/worker"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("TEST_MODE", "true"); err != nil {
		fmt.Println("Failed to set TEST_MODE:", err)
		os.Exit(1)
	}

	conf := config.Config{
		Log:        config.LogConfig{LogLevel: "ERROR"},
		DataSource: config.DataSourceConfig{Type: database.TypeSQLite},
	}
	config.OverrideCDSRuntime(conf)
	_ = log.Init("ERROR")

	testDB, err := setup.SetupTestSQLite()
	if err != nil {
		fmt.Println("Failed to start the inbuilt test database:", err)
		os.Exit(1)
	}
	provider.SetTestDB(testDB.DB, database.TypeSQLite)

	if err := workers.StartProfileWorker(); err != nil {
		fmt.Println("Failed to start profile worker:", err)
		os.Exit(1)
	}
	workers.RegisterFuzzyResolveFunc(irWorker.ResolveProfileAsync)
	workers.RegisterReindexAfterMergeFunc(irWorker.ReindexAfterMerge)

	code := m.Run()

	_ = workers.StopProfileWorker()
	testDB.Terminate()
	os.Exit(code)
}
