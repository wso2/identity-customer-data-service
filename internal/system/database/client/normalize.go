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

package client

import (
	"strings"
	"time"
)

// The stores read query results out of []map[string]interface{} with direct type
// assertions, which means the concrete Go type each column arrives as is part of
// the contract. lib/pq and modernc.org/sqlite disagree on several of those types:
//
//	column type   lib/pq       modernc.org/sqlite
//	-----------   ----------   ------------------
//	JSONB         []byte       string
//	BOOLEAN       bool         int64 (0/1)
//	TIMESTAMP     time.Time    time.Time, or text if the column is not declared
//	              	           TIMESTAMP/DATETIME
//
// The helpers below coerce SQLite results to the lib/pq shapes so the stores
// need no engine-specific branching. They are driven by the column's *declared*
// type, which SQLite reports through ColumnTypes, so adding a column to
// dbscripts/sqlite.sql needs no change here as long as it is declared with one of
// the types recognised below.
//
// Every coercion is idempotent and nil-safe: a value that already has the target
// type, or is NULL, is returned unchanged.

// sqliteTimeLayouts are the formats the inbuilt database stores timestamps in.
//
// The first covers both driver-written values (`_time_format=sqlite`, variable
// sub-second precision) and the schema's strftime defaults; it also matches
// values with no sub-second part at all. The second is a defensive fallback for
// databases written before `_time_format=sqlite` was set, where the driver used
// time.Time.String().
var sqliteTimeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999 -0700 MST",
	time.RFC3339Nano,
}

// normalizeSQLiteValue coerces a single scanned value based on its declared
// column type.
func normalizeSQLiteValue(value interface{}, declaredType string) interface{} {

	if value == nil {
		return nil
	}

	switch sqliteColumnClass(declaredType) {
	case columnClassJSON:
		// The stores unmarshal these columns from []byte, as lib/pq returns.
		if text, ok := value.(string); ok {
			return []byte(text)
		}
	case columnClassBool:
		// SQLite has no boolean storage class; it returns 0/1.
		switch typed := value.(type) {
		case int64:
			return typed != 0
		case float64:
			return typed != 0
		case string:
			return parseSQLiteBool(typed)
		}
	case columnClassTime:
		// Declaring the column TIMESTAMP makes the driver return time.Time, so
		// this is a fallback for other declarations and for text values written
		// by an older configuration.
		switch typed := value.(type) {
		case string:
			if parsed, ok := parseSQLiteTime(typed); ok {
				return parsed
			}
		case []byte:
			if parsed, ok := parseSQLiteTime(string(typed)); ok {
				return parsed
			}
		}
	}

	return value
}

// columnClass identifies the groups of declared column types that need coercion.
type columnClass int

const (
	columnClassOther columnClass = iota
	columnClassJSON
	columnClassBool
	columnClassTime
)

// sqliteColumnClass maps a declared column type to its coercion class. The
// declared types come from dbscripts/sqlite.sql.
func sqliteColumnClass(declaredType string) columnClass {

	switch strings.ToUpper(strings.TrimSpace(declaredType)) {
	case "JSON", "JSONB":
		return columnClassJSON
	case "BOOL", "BOOLEAN":
		return columnClassBool
	case "TIMESTAMP", "TIMESTAMPTZ", "DATETIME", "DATE":
		return columnClassTime
	default:
		return columnClassOther
	}
}

// parseSQLiteTime parses a timestamp stored by the inbuilt database.
func parseSQLiteTime(value string) (time.Time, bool) {

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}

	for _, layout := range sqliteTimeLayouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC(), true
		}
	}

	return time.Time{}, false
}

// parseSQLiteBool interprets the textual boolean spellings SQLite may hold.
func parseSQLiteBool(value string) bool {

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes":
		return true
	default:
		return false
	}
}

// normalizeSQLiteArgs prepares query arguments for the inbuilt database.
//
// The stores pass marshalled JSON as []byte, which binds as a BLOB. SQLite's
// JSON functions treat a BLOB argument as the binary JSONB encoding rather than
// JSON text, and a BLOB is opaque when inspecting the file with the sqlite3 CLI.
// Binding these as text avoids both problems.
func normalizeSQLiteArgs(args []interface{}) []interface{} {

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
