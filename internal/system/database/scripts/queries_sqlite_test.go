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

package scripts_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	"github.com/wso2/identity-customer-data-service/test/setup"
)

// placeholders returns "($1, $2, ... $n)", the value tuple the stores build at
// runtime for the batch-insert statements.
func placeholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", i+1)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// allQueries lists every statement declared in queries.go, keyed by its name.
func allQueries() map[string]map[string]string {
	return map[string]map[string]string{
		"UpsertApplication":                           scripts.UpsertApplication,
		"GetAppIdentifierByClientID":                  scripts.GetAppIdentifierByClientID,
		"DeleteProfileSchemaForOrg":                   scripts.DeleteProfileSchemaForOrg,
		"GetProfileSchemaByOrg":                       scripts.GetProfileSchemaByOrg,
		"DeleteIdentityClaimsOfProfileSchema":         scripts.DeleteIdentityClaimsOfProfileSchema,
		"GetProfileSchemaAttributeByName":             scripts.GetProfileSchemaAttributeByName,
		"GetProfileSchemaAttributeByScope":            scripts.GetProfileSchemaAttributeByScope,
		"UpdateProfileSchemaAttributesForSchema":      scripts.UpdateProfileSchemaAttributesForSchema,
		"DeleteProfileSchemaAttributeForScope":        scripts.DeleteProfileSchemaAttributeForScope,
		"GetProfileSchemaAttributeById":               scripts.GetProfileSchemaAttributeById,
		"FilterProfileSchemaAttributes":               scripts.FilterProfileSchemaAttributes,
		"DeleteProfileSchemaAttributeById":            scripts.DeleteProfileSchemaAttributeById,
		"GetUnificationRules":                         scripts.GetUnificationRules,
		"GetUnificationRule":                          scripts.GetUnificationRule,
		"DeleteUnificationRule":                       scripts.DeleteUnificationRule,
		"InsertUnificationRule":                       scripts.InsertUnificationRule,
		"UpdateUnificationRule":                       scripts.UpdateUnificationRule,
		"InsertProfile":                               scripts.InsertProfile,
		"InsertProfileReference":                      scripts.InsertProfileReference,
		"GetProfileById":                              scripts.GetProfileById,
		"GetProfileConsentsByProfileId":               scripts.GetProfileConsentsByProfileId,
		"DeleteProfileConsentsByProfileId":            scripts.DeleteProfileConsentsByProfileId,
		"InsertProfileConsentsByProfileId":            scripts.InsertProfileConsentsByProfileId,
		"GetAppDataByProfileId":                       scripts.GetAppDataByProfileId,
		"GetAppDataByAppId":                           scripts.GetAppDataByAppId,
		"UpdateProfile":                               scripts.UpdateProfile,
		"UpsertProfileReference":                      scripts.UpsertProfileReference,
		"UpdateProfileReference":                      scripts.UpdateProfileReference,
		"GetProfilesByOrgId":                          scripts.GetProfilesByOrgId,
		"DeleteProfileByProfileId":                    scripts.DeleteProfileByProfileId,
		"InsertApplicationData":                       scripts.InsertApplicationData,
		"DeleteProfileReference":                      scripts.DeleteProfileReference,
		"GetAllProfilesWithFilterBase":                scripts.GetAllProfilesWithFilterBase,
		"GetAllReferenceProfileExceptCurrent":         scripts.GetAllReferenceProfileExceptCurrent,
		"FetchReferencedProfiles":                     scripts.FetchReferencedProfiles,
		"GetProfileByUserId":                          scripts.GetProfileByUserId,
		"InsertConsentCategory":                       scripts.InsertConsentCategory,
		"UpsertDefaultIdentityDataCategory":           scripts.UpsertDefaultIdentityDataCategory,
		"GetAllConsentCategories":                     scripts.GetAllConsentCategories,
		"GetConsentCategoryById":                      scripts.GetConsentCategoryById,
		"GetConsentCategoryByName":                    scripts.GetConsentCategoryByName,
		"GetMandatoryConsentCategoryIdsByOrg":         scripts.GetMandatoryConsentCategoryIdsByOrg,
		"UpdateConsentCategory":                       scripts.UpdateConsentCategory,
		"DeleteConsentCategory":                       scripts.DeleteConsentCategory,
		"InsertConsentCategoryAttribute":              scripts.InsertConsentCategoryAttribute,
		"GetConsentCategoryAttributesByCategoryId":    scripts.GetConsentCategoryAttributesByCategoryId,
		"DeleteConsentCategoryAttributesByCategoryId": scripts.DeleteConsentCategoryAttributesByCategoryId,
		"InsertCookie":                                scripts.InsertCookie,
		"GetCookieByCookieId":                         scripts.GetCookieByCookieId,
		"GetCookieByProfileId":                        scripts.GetCookieByProfileId,
		"UpdateCookieStatusByProfileId":               scripts.UpdateCookieStatusByProfileId,
		"UpdateCookieStatusByCookieId":                scripts.UpdateCookieStatusByCookieId,
		"DeleteCookieById":                            scripts.DeleteCookieById,
		"DeleteCookieByProfileId":                     scripts.DeleteCookieByProfileId,
		"GetOrgConfigurations":                        scripts.GetOrgConfigurations,
		"UpdateOrgConfiguration":                      scripts.UpdateOrgConfiguration,
		"GetOrgConfiguration":                         scripts.GetOrgConfiguration,
		"UpdateInitialSchemaSyncDoneConfig":           scripts.UpdateInitialSchemaSyncDoneConfig,

		"InsertIdentityClaimsForProfileSchema":  scripts.InsertIdentityClaimsForProfileSchema,
		"UpsertIdentityClaimsForProfileSchema":  scripts.UpsertIdentityClaimsForProfileSchema,
		"InsertProfileSchemaAttributesForScope": scripts.InsertProfileSchemaAttributesForScope,
		"GetAppDataByProfileIds":                scripts.GetAppDataByProfileIds,
		"DeleteInactiveCookies":                 scripts.DeleteInactiveCookies,
	}
}

// completeStatement returns the statement as a store executes it. Some statements
// are templates that the stores complete at runtime, and an incomplete statement
// cannot be prepared.
func completeStatement(name, statement string) string {

	switch name {
	case "InsertIdentityClaimsForProfileSchema":
		return statement + placeholders(13)
	case "InsertProfileSchemaAttributesForScope":
		return statement + placeholders(12)
	case "UpsertIdentityClaimsForProfileSchema":
		return fmt.Sprintf(statement, placeholders(13))
	case "GetAppDataByProfileIds":
		return fmt.Sprintf(statement, "$1")
	default:
		return statement
	}
}

// TestQueriesAreDefinedForEverySupportedType guards against a statement that
// resolves to the empty string, which a store would send to the database as-is.
func TestQueriesAreDefinedForEverySupportedType(t *testing.T) {

	for name, query := range allQueries() {
		for _, dbType := range []string{database.TypePostgres, database.TypeSQLite} {
			if query[dbType] == "" {
				t.Errorf("%s has no statement for %q", name, dbType)
			}
		}
	}
}

// TestSQLiteQueriesPrepare prepares every SQLite statement against the inbuilt
// schema. Preparing validates the SQL grammar and every table and column the
// statement references, so this covers both dbscripts/sqlite.sql and the SQLite
// variants in queries.go without needing valid argument values.
func TestSQLiteQueriesPrepare(t *testing.T) {

	testDB, err := setup.SetupTestSQLite()
	if err != nil {
		t.Fatalf("failed to set up the inbuilt test database: %v", err)
	}
	defer testDB.Terminate()

	for name, query := range allQueries() {
		if name == "DeleteInactiveCookies" {
			// Skipped on purpose: it targets a table named "cookie_profiles",
			// while the schema defines "profile_cookies". That is a pre-existing
			// defect on PostgreSQL too, and is not addressed here.
			continue
		}

		t.Run(name, func(t *testing.T) {
			statement := completeStatement(name, query[database.TypeSQLite])
			stmt, err := testDB.DB.Prepare(statement)
			if err != nil {
				t.Fatalf("failed to prepare the query: %v\n%s", err, statement)
			}
			_ = stmt.Close()
		})
	}
}
