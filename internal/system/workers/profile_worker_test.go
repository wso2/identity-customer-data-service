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

package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/test/setup"
)

func TestUnificationRuleValues(t *testing.T) {
	var p profileModel.Profile
	require.NoError(t, json.Unmarshal([]byte(`{"identity_attributes":{
		"contact":[{"email":["A","",123,"A"]},{"email":"B"}],
		"nested":[["not-a-string-leaf"]],"number":123,"null":null},
		"traits":{"level":{"tag":"x"}}}`), &p))
	require.Equal(t, []string{"A", "", "B"}, unificationRuleValues(p, "identity_attributes.contact.email"))
	require.Equal(t, []string{"x"}, unificationRuleValues(p, "traits.level.tag"))
	for _, path := range []string{"identity_attributes.nested", "identity_attributes.number",
		"identity_attributes.null", "identity_attributes.missing", "application_data.email"} {
		require.Empty(t, unificationRuleValues(p, path), path)
	}
}

func candidateWorkerDB(t *testing.T) *sql.DB {
	t.Helper()
	require.NoError(t, log.Init("ERROR"))
	db, err := setup.SetupTestSQLite()
	require.NoError(t, err)
	provider.SetTestDB(db.DB, database.TypeSQLite)
	t.Cleanup(func() { provider.SetTestDB(nil, ""); db.Terminate() })
	return db.DB
}

func workerProfile(t *testing.T, db *sql.DB, id, userID, attrs string, age time.Duration) profileModel.Profile {
	t.Helper()
	created := time.Now().UTC().Add(-age)
	_, err := db.Exec(`INSERT INTO profiles
		(profile_id, org_handle, user_id, identity_attributes, created_at, location)
		VALUES (?, 'tenant', ?, ?, ?, '')`, id, userID, attrs, created)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO profile_reference (profile_id, org_handle, profile_status, reference_reason)
		VALUES (?, 'tenant', 'REFERENCE_PROFILE', '')`, id)
	require.NoError(t, err)
	p, err := profileStore.GetProfileWithOptions(context.Background(), id,
		profileStore.GetProfileOptions{IncludeApplicationData: false})
	require.NoError(t, err)
	require.NotNil(t, p)
	return *p
}

func workerRule(t *testing.T, db *sql.DB, id, property string, priority int, active bool) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO profile_schema
		(attribute_id, org_handle, attribute_name, display_name, value_type, merge_strategy,
		application_identifier, mutability, scope)
		VALUES (?, 'tenant', ?, ?, 'string', 'combine', '', 'readWrite', 'identity_attributes')`, id, property, property)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO unification_rules
		(rule_id, org_handle, rule_name, property_name, property_id, priority, is_active)
		VALUES (?, 'tenant', ?, ?, ?, ?, ?)`, id, id, property, id, priority, active)
	require.NoError(t, err)
}

func TestUnificationUserIDPrecedesRulesAndPreservesApplicationData(t *testing.T) {
	db := candidateWorkerDB(t)
	workerProfile(t, db, "rule-match", "different-user", `{"email":"shared"}`, 2*time.Hour)
	workerProfile(t, db, "user-match", "same-user", `{"email":"different"}`, time.Hour)
	incoming := workerProfile(t, db, "incoming", "same-user", `{"email":"shared"}`, 0)
	// A user-ID match must not even depend on reading the rules table.
	_, err := db.Exec(`DROP TABLE unification_rules`)
	require.NoError(t, err)
	for _, row := range []struct{ profile, app, data string }{
		{"user-match", "existing-app", `{"app_specific_data":{"language":"en"}}`},
		{"incoming", "new-app", `{"app_specific_data":{"theme":"dark"}}`},
	} {
		_, err = db.Exec(`INSERT INTO application_data (profile_id, app_id, application_data) VALUES (?, ?, ?)`, row.profile, row.app, row.data)
		require.NoError(t, err)
	}
	unifyProfiles(context.Background(), incoming)
	var parent, reason string
	require.NoError(t, db.QueryRow(`SELECT reference_profile_id, reference_reason FROM profile_reference WHERE profile_id='incoming'`).Scan(&parent, &reason))
	require.Equal(t, "user-match", parent)
	require.Equal(t, constants.SystemUserIdMatchReason, reason)
	apps, err := profileStore.FetchApplicationData(context.Background(), "user-match")
	require.NoError(t, err)
	require.Len(t, apps, 2)
	data := map[string]interface{}{}
	for _, app := range apps {
		data[app.AppId] = app.AppSpecificData
	}
	require.Equal(t, map[string]interface{}{"language": "en"}, data["existing-app"])
	require.Equal(t, map[string]interface{}{"theme": "dark"}, data["new-app"])
}

func TestUnificationRulePriorityAndSingleMerge(t *testing.T) {
	db := candidateWorkerDB(t)
	workerRule(t, db, "phone-rule", "identity_attributes.phone", 1, true)
	workerRule(t, db, "email-rule", "identity_attributes.email", 2, true)
	workerRule(t, db, "inactive-rule", "identity_attributes.inactive", 0, false)
	workerProfile(t, db, "email-master", "email-user", `{"email":"shared","inactive":"yes"}`, 2*time.Hour)
	workerProfile(t, db, "phone-master", "phone-user", `{"phone":"123"}`, time.Hour)
	incoming := workerProfile(t, db, "incoming", "", `{"email":"shared","phone":"123","inactive":"yes"}`, 0)
	unifyProfiles(context.Background(), incoming)
	var parent, reason string
	require.NoError(t, db.QueryRow(`SELECT reference_profile_id, reference_reason FROM profile_reference WHERE profile_id='incoming'`).Scan(&parent, &reason))
	require.Equal(t, "phone-master", parent)
	require.Equal(t, "phone-rule", reason)
	var status string
	require.NoError(t, db.QueryRow(`SELECT profile_status FROM profile_reference WHERE profile_id='email-master'`).Scan(&status))
	require.Equal(t, "REFERENCE_PROFILE", status)
}

func TestUnificationSkipsConflictingUserIDCandidate(t *testing.T) {
	db := candidateWorkerDB(t)
	workerRule(t, db, "email-rule", "identity_attributes.email", 1, true)
	workerProfile(t, db, "old-conflict", "other-user", `{"email":"shared"}`, 2*time.Hour)
	workerProfile(t, db, "eligible-temp", "", `{"email":"shared"}`, time.Hour)
	incoming := workerProfile(t, db, "incoming", "incoming-user", `{"email":"shared"}`, 0)
	unifyProfiles(context.Background(), incoming)
	var status, parent, reason string
	require.NoError(t, db.QueryRow(`SELECT profile_status, reference_profile_id, reference_reason
		FROM profile_reference WHERE profile_id='eligible-temp'`).Scan(&status, &parent, &reason))
	require.Equal(t, "MERGED_TO", status)
	require.Equal(t, "incoming", parent)
	require.Equal(t, "email-rule", reason)
}

func TestUnificationChildStaysWithParent(t *testing.T) {
	db := candidateWorkerDB(t)
	workerProfile(t, db, "parent", "parent-user", `{"email":"shared"}`, time.Hour)
	workerProfile(t, db, "incoming", "incoming-user", `{"email":"shared"}`, 0)
	_, err := db.Exec(`UPDATE profile_reference SET profile_status='MERGED_TO', reference_profile_id='parent'
		WHERE profile_id='incoming'`)
	require.NoError(t, err)
	child, err := profileStore.GetProfileWithOptions(context.Background(), "incoming",
		profileStore.GetProfileOptions{IncludeApplicationData: false})
	require.NoError(t, err)
	unifyProfiles(context.Background(), *child)
	var parentID string
	require.NoError(t, db.QueryRow(`SELECT reference_profile_id FROM profile_reference
		WHERE profile_id='incoming'`).Scan(&parentID))
	require.Equal(t, "parent", parentID)
}

func TestUnificationWaitingProfileIsSkipped(t *testing.T) {
	db := candidateWorkerDB(t)
	incoming := workerProfile(t, db, "incoming", "incoming-user", `{"email":"shared"}`, 0)
	_, err := db.Exec(`UPDATE profile_reference SET profile_status='WAIT_ON_USER' WHERE profile_id='incoming'`)
	require.NoError(t, err)
	incoming.ProfileStatus.IsReferenceProfile = false
	incoming.ProfileStatus.IsWaitingOnUser = true
	unifyProfiles(context.Background(), incoming)
	var status string
	require.NoError(t, db.QueryRow(`SELECT profile_status FROM profile_reference
		WHERE profile_id='incoming'`).Scan(&status))
	require.Equal(t, "WAIT_ON_USER", status)
}

func TestCandidateDiscoveryDoesNotRequireApplicationDataTable(t *testing.T) {
	db := candidateWorkerDB(t)
	workerRule(t, db, "email-rule", "identity_attributes.email", 1, true)
	workerProfile(t, db, "master", "user", `{"email":"a"}`, time.Hour)
	incoming := workerProfile(t, db, "incoming", "", `{"email":"b"}`, 0)
	_, err := db.Exec(`DROP TABLE application_data`)
	require.NoError(t, err)
	_, err = profileStore.GetProfileWithOptions(context.Background(), incoming.ProfileId,
		profileStore.GetProfileOptions{IncludeApplicationData: false})
	require.NoError(t, err)
	id, err := profileStore.FindOldestReferenceProfileIDByAttributeValues(context.Background(), incoming.OrgHandle,
		"identity_attributes.email", []string{"a"}, incoming.ProfileId, incoming.UserId)
	require.NoError(t, err)
	require.Equal(t, "master", id)
	unifyProfiles(context.Background(), incoming)
	// A selected pair needs application data. A read failure must leave the
	// relationship unchanged rather than merging an incomplete profile.
	incoming.IdentityAttributes["email"] = "a"
	unifyProfiles(context.Background(), incoming)
	var status string
	require.NoError(t, db.QueryRow(`SELECT profile_status FROM profile_reference WHERE profile_id='incoming'`).Scan(&status))
	require.Equal(t, "REFERENCE_PROFILE", status)
}
