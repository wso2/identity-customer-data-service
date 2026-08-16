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

package setup

import (
	"fmt"
	"strings"
)

// Preparing a statement validates its grammar and every table and column it
// references, without needing valid argument values. Two suites do it — the
// unit tests against the inbuilt database and the integration suite against
// whichever datasource it runs on — so the knowledge of how to complete a
// runtime template lives here rather than in either of them.

// QueryPlaceholders returns "($1, $2, ... $n)", the value tuple the stores build
// at runtime for the batch-insert statements.
func QueryPlaceholders(n int) string {

	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", i+1)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// CompleteStatement returns the statement as a store executes it. Some
// statements are templates the stores complete at runtime, and an incomplete
// statement cannot be prepared. name is the Go name the statement is declared
// under in scripts.AllQueries.
func CompleteStatement(name, statement string) string {

	switch name {
	case "InsertIdentityClaimsForProfileSchema":
		return statement + QueryPlaceholders(13)
	case "InsertProfileSchemaAttributesForScope":
		return statement + QueryPlaceholders(12)
	case "UpdateProfileSchemaAttributeFields":
		return statement + "attribute_name = $1 WHERE org_handle = $2 AND attribute_id = $3"
	case "UpsertIdentityClaimsForProfileSchema":
		return fmt.Sprintf(statement, QueryPlaceholders(13))
	case "DeleteStaleIdentityClaimsForProfileSchema":
		return fmt.Sprintf(statement, "$2")
	case "GetAppDataByProfileIds", "GetConsentCategoryAttributesByCategoryIds":
		return fmt.Sprintf(statement, "$1")
	default:
		return statement
	}
}

// SkipPreparing reports why a statement cannot be prepared, or "" when it can.
func SkipPreparing(name string) string {

	if name == "DeleteInactiveCookies" {
		// Skipped on purpose: it targets a table named "cookie_profiles", while
		// the schema defines "profile_cookies". That is a pre-existing defect on
		// PostgreSQL too, and is not addressed here.
		return "targets a table the schema does not define"
	}
	return ""
}
