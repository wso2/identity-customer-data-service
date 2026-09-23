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

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	dbmodel "github.com/wso2/identity-customer-data-service/internal/system/database/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
)

// UnificationTx contains every database operation used by one profile merge.
// None of its methods opens a second connection or commits independently.
type UnificationTx struct {
	tx *dbmodel.Tx
}

// HasProcessedUnificationEvent avoids re-evaluating an event that was already
// committed but reached the worker again. The transaction still claims the
// event itself, so concurrent deliveries cannot both apply it.
func HasProcessedUnificationEvent(ctx context.Context, eventID string) (bool, error) {
	client, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return false, err
	}
	defer client.Close()
	rows, err := client.ExecuteQueryContext(ctx, scripts.GetProfileUnificationEvent, eventID)
	return len(rows) != 0, err
}

// MarkUnificationEventComplete records a successful no-match evaluation.
func MarkUnificationEventComplete(ctx context.Context, eventID, profileID string) error {
	client, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return err
	}
	defer client.Close()
	tx, err := client.BeginTxContext(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, scripts.ClaimProfileUnificationEvent, eventID, profileID); err != nil {
		return err
	}
	return tx.Commit()
}

// WithUnificationTransaction claims the event, locks both profiles in stable
// order, and commits the callback's changes and the claim together. A duplicate
// event returns applied=false. PostgreSQL row locks serialize competing merges
// of either profile; SQLite's single-writer transaction rejects conflicting
// writes rather than silently overwriting them.
func WithUnificationTransaction(ctx context.Context, eventID, existingID, incomingID string,
	apply func(*UnificationTx, model.Profile, model.Profile) error) (applied bool, err error) {
	if eventID == "" || existingID == "" || incomingID == "" || existingID == incomingID {
		return false, fmt.Errorf("invalid profile unification event or profile IDs")
	}
	client, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return false, err
	}
	defer client.Close()
	tx, err := client.BeginTxContext(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	claim, err := tx.ExecContext(ctx, scripts.ClaimProfileUnificationEvent, eventID, incomingID)
	if err != nil {
		return false, err
	}
	claimed, err := claim.RowsAffected()
	if err != nil {
		return false, err
	}
	if claimed == 0 {
		return false, nil
	}

	mergeTx := &UnificationTx{tx: tx}
	ids := []string{existingID, incomingID}
	sort.Strings(ids)
	profiles := make(map[string]model.Profile, 2)
	for _, id := range ids {
		profile, err := mergeTx.loadProfile(ctx, id)
		if err != nil {
			return false, fmt.Errorf("load profile %s for unification: %w", id, err)
		}
		profiles[id] = profile
	}
	if profiles[existingID].OrgHandle != profiles[incomingID].OrgHandle {
		return false, fmt.Errorf("cannot unify profiles in different organizations")
	}
	if err := apply(mergeTx, profiles[existingID], profiles[incomingID]); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit profile unification: %w", err)
	}
	return true, nil
}

func (u *UnificationTx) loadProfile(ctx context.Context, profileID string) (model.Profile, error) {
	rows, err := u.tx.QueryContext(ctx, scripts.LockProfileForUnification, profileID)
	if err != nil {
		return model.Profile{}, err
	}
	var p model.Profile
	var userID, location, status, parentID, reason sql.NullString
	var traitsJSON, identityJSON []byte
	var listProfile, deleteProfile bool
	if !rows.Next() {
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return p, err
		}
		return p, sql.ErrNoRows
	}
	err = rows.Scan(&p.ProfileId, &userID, &p.OrgHandle, &p.CreatedAt, &p.UpdatedAt,
		&location, &listProfile, &deleteProfile, &traitsJSON, &identityJSON,
		&status, &parentID, &reason)
	closeErr := rows.Close()
	if err != nil {
		return p, err
	}
	if closeErr != nil {
		return p, closeErr
	}
	p.UserId, p.Location = userID.String, location.String
	p.ProfileStatus = &model.ProfileStatus{
		IsReferenceProfile: status.String == constants.ReferenceProfile,
		IsWaitingOnAdmin:   status.String == constants.WaitOnAdmin,
		IsWaitingOnUser:    status.String == constants.WaitOnUser,
		ReferenceProfileId: parentID.String,
		ReferenceReason:    reason.String,
		ListProfile:        listProfile,
		DeleteProfile:      deleteProfile,
	}
	if err := json.Unmarshal(traitsJSON, &p.Traits); err != nil {
		return p, fmt.Errorf("decode traits for %s: %w", profileID, err)
	}
	if err := json.Unmarshal(identityJSON, &p.IdentityAttributes); err != nil {
		return p, fmt.Errorf("decode identity attributes for %s: %w", profileID, err)
	}
	p.ApplicationData, err = u.loadApplicationData(ctx, profileID)
	return p, err
}

func (u *UnificationTx) loadApplicationData(ctx context.Context, profileID string) ([]model.ApplicationData, error) {
	rows, err := u.tx.QueryContext(ctx, scripts.LockApplicationDataForUnification, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	apps := make([]model.ApplicationData, 0)
	for rows.Next() {
		var app model.ApplicationData
		var data []byte
		if err := rows.Scan(&app.AppId, &data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &app); err != nil {
			return nil, fmt.Errorf("decode application %s for %s: %w", app.AppId, profileID, err)
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

// FetchReferencedProfiles reads children through the merge transaction.
func (u *UnificationTx) FetchReferencedProfiles(ctx context.Context, masterID string) ([]model.Reference, error) {
	rows, err := u.tx.QueryContext(ctx, scripts.FetchReferencedProfiles, masterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := make([]model.Reference, 0)
	for rows.Next() {
		var ref model.Reference
		var reason, status sql.NullString
		if err := rows.Scan(&ref.ProfileId, &reason, &status); err != nil {
			return nil, err
		}
		ref.Reason = reason.String
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// UpdateProfileReferences changes child relationships without committing them
// before the associated master data is saved.
func (u *UnificationTx) UpdateProfileReferences(ctx context.Context, parent model.Profile,
	children []model.Reference, expectedParentID string) error {
	for _, child := range children {
		result, err := u.tx.ExecContext(ctx, scripts.UpdateUnificationReference,
			parent.ProfileId, child.Reason, constants.MergedTo, child.ProfileId,
			parent.OrgHandle, expectedParentID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("expected one reference row for child %s, updated %d", child.ProfileId, affected)
		}
	}
	return nil
}

// InsertMaster creates the neutral master used when two temporary profiles
// have no existing hierarchy.
func (u *UnificationTx) InsertMaster(ctx context.Context, master model.Profile) error {
	traits, err := json.Marshal(master.Traits)
	if err != nil {
		return err
	}
	identity, err := json.Marshal(master.IdentityAttributes)
	if err != nil {
		return err
	}
	result, err := u.tx.ExecContext(ctx, scripts.InsertProfile, master.ProfileId, master.UserId,
		master.OrgHandle, master.CreatedAt, master.UpdatedAt, master.Location,
		master.ProfileStatus.ListProfile, false, traits, identity)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("neutral master %s already exists", master.ProfileId)
	}
	result, err = u.tx.ExecContext(ctx, scripts.InsertProfileReference, master.ProfileId,
		constants.ReferenceProfile, "", "", master.OrgHandle, master.OrgHandle)
	if err != nil {
		return err
	}
	affected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("neutral master reference %s already exists", master.ProfileId)
	}
	return nil
}

// SaveMergedMaster writes all merged data on the same connection and in the
// same transaction as the relationship updates and event claim.
func (u *UnificationTx) SaveMergedMaster(ctx context.Context, master model.Profile) error {
	traits, err := json.Marshal(master.Traits)
	if err != nil {
		return err
	}
	identity, err := json.Marshal(master.IdentityAttributes)
	if err != nil {
		return err
	}
	result, err := u.tx.ExecContext(ctx, scripts.UpdateProfile, master.UserId,
		master.ProfileStatus.ListProfile, master.ProfileStatus.DeleteProfile,
		traits, identity, time.Now().UTC(), master.ProfileId)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("merged master %s is missing", master.ProfileId)
	}
	for _, app := range master.ApplicationData {
		data, err := json.Marshal(struct {
			AppSpecificData map[string]interface{} `json:"app_specific_data,omitempty"`
		}{AppSpecificData: app.AppSpecificData})
		if err != nil {
			return err
		}
		if _, err := u.tx.ExecContext(ctx, scripts.InsertApplicationData,
			master.ProfileId, app.AppId, data); err != nil {
			return err
		}
	}
	return nil
}
