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

package model

import "time"

// Organization status values.
const (
	StatusActive   = "ACTIVE"
	StatusDisabled = "DISABLED"
	StatusDeleted  = "DELETED"
)

// Organization is the CDS view of one org of the identity provider. CDS keys an org by the
// stable org ID of the identity provider. The handle is an alias for routing.
type Organization struct {
	OrgId        string    `json:"org_id"`
	OrgHandle    string    `json:"org_handle"`
	OrgName      string    `json:"org_name,omitempty"`
	ParentOrgId  string    `json:"parent_org_id,omitempty"`
	PrimaryOrgId string    `json:"primary_org_id"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IsPrimary reports whether the org is the primary org of its organization tree.
func (o Organization) IsPrimary() bool {
	return o.OrgId == o.PrimaryOrgId
}

// SyncEvent is an org lifecycle event that the identity provider pushes to CDS.
type SyncEvent struct {
	Event       string `json:"event"`
	OrgId       string `json:"org_id"`
	ParentOrgId string `json:"parent_org_id,omitempty"`
	OrgHandle   string `json:"org_handle,omitempty"`
	OrgName     string `json:"org_name,omitempty"`
}

// Org lifecycle event names that CDS accepts. They are neutral to the identity provider. The IS
// extension maps the IS event names to them.
const (
	EventOrgCreated = "ORG_CREATED"
	EventOrgUpdated = "ORG_UPDATED"
	EventOrgDeleted = "ORG_DELETED"
)

// Tree sync states of a primary org. NOT_SYNCED means that no tree sync started yet.
const (
	TreeSyncNotSynced = "NOT_SYNCED"
	TreeSyncSyncing   = "SYNCING"
	TreeSyncSynced    = "SYNCED"
	TreeSyncFailed    = "FAILED"
)

// The cds_config keys of the primary org that hold the tree sync state.
const (
	ConfigTreeSyncState      = "tree_sync_state"
	ConfigTreeSyncStartedAt  = "tree_sync_started_at"
	ConfigTreeSyncFinishedAt = "tree_sync_finished_at"
	ConfigTreeSyncedAt       = "tree_synced_at"
	ConfigTreeSyncOrgCount   = "tree_sync_org_count"
	ConfigTreeSyncError      = "tree_sync_error"
)

// TreeSyncStatus is the state of the tree sync of a primary org. LastSyncedAt is the end of the
// last sync that completed. Organization access needs it.
type TreeSyncStatus struct {
	State        string     `json:"state"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	OrgCount     *int       `json:"org_count,omitempty"`
	Error        string     `json:"error,omitempty"`
}
