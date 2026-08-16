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
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	"github.com/wso2/identity-customer-data-service/test/setup"
)

// Test_QueriesPrepare prepares every declared statement against the datasource
// the suite is running on, which is a PostgreSQL container by default.
//
// Preparing validates the SQL grammar and every table and column the statement
// references, so this catches a statement that names a column the schema does
// not have — the failure mode the SQLite overrides introduce, since only the
// dialect that a suite exercises would otherwise be checked. The unit tests
// cover the SQLite text; this covers whichever text the suite resolves.
func Test_QueriesPrepare(t *testing.T) {

	for name, query := range scripts.AllQueries() {
		t.Run(name, func(t *testing.T) {
			if reason := setup.SkipPreparing(name); reason != "" {
				t.Skip(reason)
			}

			statement := setup.CompleteStatement(name, query.GetQuery(suiteDBType))
			stmt, err := suiteDB.Prepare(statement)
			if err != nil {
				t.Fatalf("failed to prepare the query: %v\n%s", err, statement)
			}
			_ = stmt.Close()
		})
	}
}
