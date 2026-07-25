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

// Package dbscripts embeds the database schema scripts into the binary.
//
// Only the SQLite script is embedded: it is applied automatically at startup so
// the inbuilt database needs no external tooling. postgres.sql stays an
// operator-applied artifact, since creating a PostgreSQL schema requires
// privileges (CREATE EXTENSION pg_trgm) that the service is not assumed to have.
package dbscripts

import _ "embed"

// SQLiteSchema is the DDL for the inbuilt SQLite database. Every statement is
// idempotent, so it can be applied to an existing database safely.
//
//go:embed sqlite.sql
var SQLiteSchema string
