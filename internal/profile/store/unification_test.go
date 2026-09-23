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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/database"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/test/setup"
)

func newUnificationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	testDB, err := setup.SetupTestSQLite()
	require.NoError(t, err)
	provider.SetTestDB(testDB.DB, database.TypeSQLite)
	t.Cleanup(func() {
		provider.SetTestDB(nil, "")
		testDB.Terminate()
	})
	now := time.Now().UTC()
	for _, id := range []string{"master", "incoming"} {
		_, err := testDB.DB.Exec(`INSERT INTO profiles
			(profile_id, user_id, org_handle, created_at, updated_at, location,
			 list_profile, delete_profile, traits, identity_attributes)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, "", "tenant", now, now, "/profiles/"+id, true, false, `{}`, `{}`)
		require.NoError(t, err)
		_, err = testDB.DB.Exec(`INSERT INTO profile_reference
			(profile_id, org_handle, profile_status) VALUES (?, ?, ?)`,
			id, "tenant", "REFERENCE_PROFILE")
		require.NoError(t, err)
	}
	return testDB.DB
}

func TestUnificationTransactionRollsBackEveryWriteAndClaim(t *testing.T) {
	db := newUnificationTestDB(t)
	ctx := context.Background()
	injected := errors.New("later merge write failed")
	applied, err := store.WithUnificationTransaction(ctx, "event-rollback", "master", "incoming",
		func(tx *store.UnificationTx, master, incoming model.Profile) error {
			require.Equal(t, "master", master.ProfileId)
			require.Equal(t, "incoming", incoming.ProfileId)
			require.NoError(t, tx.UpdateProfileReferences(ctx, master,
				[]model.Reference{{ProfileId: incoming.ProfileId, Reason: "test"}}, ""))
			master.Traits = map[string]interface{}{"merged": true}
			master.ApplicationData = []model.ApplicationData{{AppId: "app", AppSpecificData: map[string]interface{}{"x": "y"}}}
			require.NoError(t, tx.SaveMergedMaster(ctx, master))
			return injected
		})
	require.False(t, applied)
	require.ErrorIs(t, err, injected)

	var status string
	var parent sql.NullString
	require.NoError(t, db.QueryRow(`SELECT profile_status, reference_profile_id FROM profile_reference
		WHERE profile_id = 'incoming'`).Scan(&status, &parent))
	require.Equal(t, "REFERENCE_PROFILE", status)
	require.False(t, parent.Valid)
	var traits string
	require.NoError(t, db.QueryRow(`SELECT traits FROM profiles WHERE profile_id = 'master'`).Scan(&traits))
	require.Equal(t, `{}`, traits)
	var appCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM application_data`).Scan(&appCount))
	require.Zero(t, appCount)
	processed, err := store.HasProcessedUnificationEvent(ctx, "event-rollback")
	require.NoError(t, err)
	require.False(t, processed)
}

func TestUnificationTransactionSkipsCommittedRedelivery(t *testing.T) {
	db := newUnificationTestDB(t)
	ctx := context.Background()
	callbackCalls := 0
	apply := func(tx *store.UnificationTx, master, incoming model.Profile) error {
		callbackCalls++
		master.Traits = map[string]interface{}{"merged": true}
		if err := tx.UpdateProfileReferences(ctx, master,
			[]model.Reference{{ProfileId: incoming.ProfileId, Reason: "test"}}, ""); err != nil {
			return err
		}
		return tx.SaveMergedMaster(ctx, master)
	}
	applied, err := store.WithUnificationTransaction(ctx, "event-replay", "master", "incoming", apply)
	require.NoError(t, err)
	require.True(t, applied)
	applied, err = store.WithUnificationTransaction(ctx, "event-replay", "master", "incoming", apply)
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, 1, callbackCalls)

	var parent, status string
	require.NoError(t, db.QueryRow(`SELECT reference_profile_id, profile_status FROM profile_reference
		WHERE profile_id = 'incoming'`).Scan(&parent, &status))
	require.Equal(t, "master", parent)
	require.Equal(t, "MERGED_TO", status)
	processed, err := store.HasProcessedUnificationEvent(ctx, "event-replay")
	require.NoError(t, err)
	require.True(t, processed)
}

func TestUnificationTransactionRejectsChangedChildRelationship(t *testing.T) {
	db := newUnificationTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE profile_reference SET profile_status = 'MERGED_TO',
		reference_profile_id = 'other-master' WHERE profile_id = 'incoming'`)
	require.NoError(t, err)
	applied, err := store.WithUnificationTransaction(ctx, "event-conflict", "master", "incoming",
		func(tx *store.UnificationTx, master, incoming model.Profile) error {
			return tx.UpdateProfileReferences(ctx, master,
				[]model.Reference{{ProfileId: incoming.ProfileId, Reason: "test"}}, "")
		})
	require.False(t, applied)
	require.ErrorContains(t, err, "updated 0")
	var parent string
	require.NoError(t, db.QueryRow(`SELECT reference_profile_id FROM profile_reference
		WHERE profile_id = 'incoming'`).Scan(&parent))
	require.Equal(t, "other-master", parent)
	processed, err := store.HasProcessedUnificationEvent(ctx, "event-conflict")
	require.NoError(t, err)
	require.False(t, processed)
}
