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

package scripts

import "github.com/wso2/identity-customer-data-service/internal/system/database"

// newQuery builds a statement keyed by datasource type, as returned by
// DBProvider.GetDBType.
//
// base is the PostgreSQL statement and is also used for every other supported
// datasource unless an override is supplied. Most statements need no override:
// SQLite accepts `$N` placeholders, `ON CONFLICT ... DO UPDATE` and `EXCLUDED`,
// and row-value comparisons. Pass sqliteOverride only where the dialects
// genuinely differ — PostgreSQL `::` casts and `now()` are the cases in
// practice.
//
// Populating every supported dialect here is what keeps a lookup from silently
// resolving to an empty statement.
func newQuery(base string, sqliteOverride ...string) map[string]string {

	sqlite := base
	if len(sqliteOverride) > 0 && sqliteOverride[0] != "" {
		sqlite = sqliteOverride[0]
	}

	return map[string]string{
		database.TypePostgres: base,
		database.TypeSQLite:   sqlite,
	}
}
