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

package database

// NormalizeSQLiteArgs prepares query arguments for the inbuilt database.
//
// The stores pass marshalled JSON as []byte, which binds as a BLOB. SQLite's
// JSON functions treat a BLOB argument as the binary JSONB encoding rather than
// JSON text, and a BLOB is opaque when inspecting the file with the sqlite3 CLI.
// Binding these as text avoids both problems.
//
// This lives in the database package rather than next to the scan-side
// normalization so that both statement paths reach it: the client for
// standalone statements and the transaction wrapper for statements inside a
// transaction. Applying it in only one of the two would store the same JSON
// column as TEXT on one path and as a BLOB on the other.
func NormalizeSQLiteArgs(args []interface{}) []interface{} {

	if len(args) == 0 {
		return args
	}

	normalized := make([]interface{}, len(args))
	for i, arg := range args {
		if raw, ok := arg.([]byte); ok {
			normalized[i] = string(raw)
			continue
		}
		normalized[i] = arg
	}

	return normalized
}
