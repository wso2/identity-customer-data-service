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

// Package model holds the types the database client and the query layer share:
// a statement in every supported dialect, and a transaction that knows which
// dialect it is running against.
//
// Both exist so that the dialect is resolved in exactly one place. A store
// names a statement and the client picks the text; the store never learns which
// engine it is talking to.
package model

import (
	"fmt"

	"github.com/wso2/identity-customer-data-service/internal/system/database"
)

// DBQuery is a statement in every supported dialect, identified for logging.
type DBQuery struct {
	// ID uniquely and permanently identifies the statement. It names the
	// statement in error messages, so a driver failure is traceable without the
	// SQL text.
	ID string
	// Query is the PostgreSQL statement and the fallback for any dialect
	// without an explicit override.
	Query string
	// SQLiteQuery overrides Query for the inbuilt datasource. Empty means the
	// PostgreSQL statement is portable as written.
	SQLiteQuery string
}

// GetQuery returns the statement for dbType. It can never return an empty
// string for a populated DBQuery, which is the property the previous
// map[string]string lookup did not have: an unrecognised key returned "" and
// the store sent an empty statement to the driver.
func (q DBQuery) GetQuery(dbType string) string {

	if dbType == database.TypeSQLite && q.SQLiteQuery != "" {
		return q.SQLiteQuery
	}
	return q.Query
}

// Format applies fmt.Sprintf to every dialect variant, for statements that are
// templates completed at runtime — a batch VALUES list or an IN list whose
// width depends on the number of rows.
//
// An empty SQLiteQuery stays empty, so a statement that has no override before
// formatting still has none afterwards and continues to fall back to the
// PostgreSQL text. Formatting it instead would leave the fallback pointing at a
// copy that only looks correct.
func (q DBQuery) Format(args ...interface{}) DBQuery {

	formatted := DBQuery{
		ID:    q.ID,
		Query: fmt.Sprintf(q.Query, args...),
	}
	if q.SQLiteQuery != "" {
		formatted.SQLiteQuery = fmt.Sprintf(q.SQLiteQuery, args...)
	}
	return formatted
}

// Append concatenates a fragment onto every dialect variant, for statements
// built by string concatenation rather than Sprintf. It preserves the empty
// SQLiteQuery fallback for the same reason Format does.
func (q DBQuery) Append(suffix string) DBQuery {

	appended := DBQuery{
		ID:    q.ID,
		Query: q.Query + suffix,
	}
	if q.SQLiteQuery != "" {
		appended.SQLiteQuery = q.SQLiteQuery + suffix
	}
	return appended
}

// WithSQL returns a copy of q whose text is sqlText for every dialect.
//
// It is for the handful of statements a store assembles from dialect-specific
// fragments — a filter whose operators and JSON accessors differ per engine —
// where the dialect has already been applied while building the text and a
// second resolution would be wrong. The ID is carried over so the assembled
// statement is still attributable to the base statement it was built from.
func (q DBQuery) WithSQL(sqlText string) DBQuery {

	return DBQuery{
		ID:    q.ID,
		Query: sqlText,
	}
}
