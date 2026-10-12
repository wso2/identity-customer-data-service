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

package service

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	adminConfigStore "github.com/wso2/identity-customer-data-service/internal/admin_config/store"
	consentService "github.com/wso2/identity-customer-data-service/internal/consent/service"
	"github.com/wso2/identity-customer-data-service/internal/organization/model"
	"github.com/wso2/identity-customer-data-service/internal/organization/store"
	sharingService "github.com/wso2/identity-customer-data-service/internal/sharing/service"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/idp"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// syncResult summarizes one sync of a customer tree.
type syncResult struct {
	Total   int
	Added   []string
	Deleted []string
}

// syncLocks serializes the sync of one primary org, so that two syncs do not write the same tree at
// the same time.
var syncLocks sync.Map

// EnsurePrimary reads the primary org from the identity provider and stores it. It does not read
// the sub orgs.
func EnsurePrimary(ctx context.Context, primaryHandle string) (*model.Organization, error) {

	primaryInfo, err := idp.GetAdapter().GetPrimaryOrganization(ctx, primaryHandle)
	if err != nil {
		return nil, err
	}
	primary := model.Organization{
		OrgId:        primaryInfo.Id,
		OrgHandle:    primaryHandle,
		OrgName:      primaryInfo.Name,
		PrimaryOrgId: primaryInfo.Id,
		Status:       model.StatusActive,
	}
	if err := store.UpsertOrganization(ctx, primary); err != nil {
		return nil, err
	}
	return &primary, nil
}

// treeSyncRun is the background sync of one primary org. again is set when a trigger comes during
// the run, so that the run reads the tree once more after it ends.
type treeSyncRun struct {
	again bool
}

var (
	treeSyncMu   sync.Mutex
	treeSyncRuns = map[string]*treeSyncRun{}
)

// StartTreeSync starts the background sync of the tree of the primary org, and returns the state.
// It returns TREE_SYNC_RUNNING when a sync of the primary org runs.
func StartTreeSync(ctx context.Context, primaryHandle string) (*model.TreeSyncStatus, error) {

	if _, err := EnsurePrimary(ctx, primaryHandle); err != nil {
		return nil, err
	}
	treeSyncMu.Lock()
	defer treeSyncMu.Unlock()
	status, err := store.GetTreeSyncStatus(ctx, primaryHandle)
	if err != nil {
		return nil, err
	}
	if status.State == model.TreeSyncSyncing {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.TREE_SYNC_RUNNING.Code,
			Message:     errors2.TREE_SYNC_RUNNING.Message,
			Description: errors2.TREE_SYNC_RUNNING.Description,
		}, http.StatusConflict)
	}
	if err := markSyncing(ctx, primaryHandle); err != nil {
		return nil, err
	}
	scheduleLocked(primaryHandle)
	return store.GetTreeSyncStatus(ctx, primaryHandle)
}

// ResetTreeSync clears the tree sync state of the primary org, so that the next sync is the first
// one again. A sync that runs ends without a stored end state.
func ResetTreeSync(ctx context.Context, primaryHandle string) error {

	treeSyncMu.Lock()
	defer treeSyncMu.Unlock()
	if run, running := treeSyncRuns[primaryHandle]; running {
		run.again = false
	}
	status, err := store.GetTreeSyncStatus(ctx, primaryHandle)
	if err != nil {
		return err
	}
	if status.State == model.TreeSyncNotSynced && status.LastSyncedAt == nil {
		return nil
	}
	return store.SetTreeSyncValues(ctx, primaryHandle, clearedTreeSyncValues())
}

// clearedTreeSyncValues returns the tree sync keys with empty values, which clear them.
func clearedTreeSyncValues() map[string]string {
	return map[string]string{
		model.ConfigTreeSyncState:      "",
		model.ConfigTreeSyncStartedAt:  "",
		model.ConfigTreeSyncFinishedAt: "",
		model.ConfigTreeSyncedAt:       "",
		model.ConfigTreeSyncOrgCount:   "",
		model.ConfigTreeSyncError:      "",
	}
}

// GetTreeSyncStatus returns the tree sync state of the primary org.
func GetTreeSyncStatus(ctx context.Context, primaryHandle string) (*model.TreeSyncStatus, error) {
	return store.GetTreeSyncStatus(ctx, primaryHandle)
}

// scheduleTreeSync starts the background sync of the primary org. When a sync of the primary org
// runs, it asks that run to read the tree once more after it ends.
func scheduleTreeSync(ctx context.Context, primaryHandle string) error {

	treeSyncMu.Lock()
	defer treeSyncMu.Unlock()
	if run, running := treeSyncRuns[primaryHandle]; running {
		run.again = true
		return nil
	}
	if err := markSyncing(ctx, primaryHandle); err != nil {
		return err
	}
	scheduleLocked(primaryHandle)
	return nil
}

// scheduleLocked starts the sync goroutine of the primary org. The caller holds treeSyncMu.
func scheduleLocked(primaryHandle string) {

	if run, running := treeSyncRuns[primaryHandle]; running {
		run.again = true
		return
	}
	run := &treeSyncRun{}
	treeSyncRuns[primaryHandle] = run
	go func() {
		for {
			runTreeSync(context.Background(), primaryHandle)
			treeSyncMu.Lock()
			if !run.again {
				delete(treeSyncRuns, primaryHandle)
				treeSyncMu.Unlock()
				return
			}
			run.again = false
			err := markSyncing(context.Background(), primaryHandle)
			treeSyncMu.Unlock()
			if err != nil {
				log.GetLogger().Error(fmt.Sprintf("Failed to store the organization sync state of primary "+
					"organization '%s'.", primaryHandle), log.Error(err))
			}
		}
	}()
}

func markSyncing(ctx context.Context, primaryHandle string) error {
	return store.SetTreeSyncValues(ctx, primaryHandle, map[string]string{
		model.ConfigTreeSyncState:      model.TreeSyncSyncing,
		model.ConfigTreeSyncStartedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		model.ConfigTreeSyncFinishedAt: "",
		model.ConfigTreeSyncError:      "",
	})
}

// runTreeSync reads the tree and stores the end state. A failure is logged and stored as FAILED.
// When the primary org disabled CDS during the run, the state is cleared.
func runTreeSync(ctx context.Context, primaryHandle string) {

	logger := log.GetLogger()
	values := map[string]string{}
	result, err := func() (result *syncResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("the organization sync stopped: %v", r)
			}
		}()
		return syncTree(ctx, primaryHandle)
	}()
	finished := time.Now().UTC().Format(time.RFC3339Nano)
	values[model.ConfigTreeSyncFinishedAt] = finished
	if err != nil {
		logger.Error(fmt.Sprintf("The organization sync of primary organization '%s' failed.", primaryHandle),
			log.Error(err))
		values[model.ConfigTreeSyncState] = model.TreeSyncFailed
		values[model.ConfigTreeSyncError] = err.Error()
	} else {
		values[model.ConfigTreeSyncState] = model.TreeSyncSynced
		values[model.ConfigTreeSyncedAt] = finished
		values[model.ConfigTreeSyncOrgCount] = strconv.Itoa(result.Total)
		values[model.ConfigTreeSyncError] = ""
	}
	treeSyncMu.Lock()
	defer treeSyncMu.Unlock()
	if config, err := adminConfigStore.GetAdminConfig(ctx, primaryHandle); err == nil &&
		(config == nil || !config.CDSEnabled) {
		values = clearedTreeSyncValues()
		if run, running := treeSyncRuns[primaryHandle]; running {
			run.again = false
		}
	}
	if err := store.SetTreeSyncValues(ctx, primaryHandle, values); err != nil {
		logger.Error(fmt.Sprintf("Failed to store the organization sync state of primary organization '%s'.",
			primaryHandle), log.Error(err))
	}
}

// syncTree reads the org tree of the primary org from the identity provider and makes the stored
// orgs match it. It initializes each new org that the organization access enables. An org that an
// event changes during the sync is not marked deleted, and a failed read from the identity provider
// deletes no org.
func syncTree(ctx context.Context, primaryHandle string) (*syncResult, error) {

	lock, _ := syncLocks.LoadOrStore(primaryHandle, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	started := time.Now().UTC()
	primary, err := EnsurePrimary(ctx, primaryHandle)
	if err != nil {
		return nil, err
	}
	existing, err := store.GetOrganizationsByPrimaryOrg(ctx, primary.OrgId)
	if err != nil {
		return nil, err
	}
	known := map[string]model.Organization{}
	for _, o := range existing {
		known[o.OrgId] = o
	}
	descendants, err := idp.GetAdapter().ListDescendants(ctx, primaryHandle)
	if err != nil {
		return nil, err
	}

	orgs := buildTree(*primary, descendants)
	result := &syncResult{Total: len(orgs) + 1, Added: []string{}, Deleted: []string{}}
	seen := map[string]bool{primary.OrgId: true}
	for _, org := range orgs {
		seen[org.OrgId] = true
		previous, wasKnown := known[org.OrgId]
		if err := store.UpsertOrganization(ctx, org); err != nil {
			return nil, err
		}
		if !wasKnown || previous.Status == model.StatusDeleted {
			initializeIfEnabled(ctx, org)
			result.Added = append(result.Added, org.OrgHandle)
		}
	}
	for _, o := range existing {
		if seen[o.OrgId] || o.Status == model.StatusDeleted {
			continue
		}
		current, err := store.GetOrganizationById(ctx, o.OrgId)
		if err != nil {
			return nil, err
		}
		if current == nil || current.Status == model.StatusDeleted || current.UpdatedAt.After(started) {
			continue
		}
		if err := store.UpdateOrganizationStatus(ctx, o.OrgId, model.StatusDeleted); err != nil {
			return nil, err
		}
		result.Deleted = append(result.Deleted, o.OrgHandle)
	}

	log.GetLogger().Info(fmt.Sprintf("Synced the org tree of primary organization '%s': %d orgs, %d added, "+
		"%d deleted.", primaryHandle, result.Total, len(result.Added), len(result.Deleted)))
	return result, nil
}

// buildTree links each descendant to its parent, and returns the orgs with each parent before its
// children, so that CDS stores a parent first. An org whose chain does not reach the primary org is
// left out.
func buildTree(primary model.Organization, descendants []idp.Organization) []model.Organization {

	byId := map[string]idp.Organization{}
	for _, d := range descendants {
		byId[d.Id] = d
	}
	built := map[string]model.Organization{primary.OrgId: primary}
	level := map[string]int{primary.OrgId: 0}
	var resolve func(id string, guard int) (model.Organization, bool)
	resolve = func(id string, guard int) (model.Organization, bool) {
		if org, ok := built[id]; ok {
			return org, true
		}
		d, ok := byId[id]
		if !ok || guard > 64 {
			return model.Organization{}, false
		}
		parent, ok := resolve(d.ParentId, guard+1)
		if !ok {
			return model.Organization{}, false
		}
		status := model.StatusActive
		if d.Status == model.StatusDisabled {
			status = model.StatusDisabled
		}
		org := model.Organization{
			OrgId:        d.Id,
			OrgHandle:    d.Handle,
			OrgName:      d.Name,
			ParentOrgId:  parent.OrgId,
			PrimaryOrgId: primary.OrgId,
			Status:       status,
		}
		built[id] = org
		level[id] = level[parent.OrgId] + 1
		return org, true
	}

	result := make([]model.Organization, 0, len(descendants))
	for _, d := range descendants {
		if org, ok := resolve(d.Id, 0); ok {
			result = append(result, org)
		} else {
			log.GetLogger().Warn(fmt.Sprintf("Skipping organization '%s', because its parent chain does not reach "+
				"the primary organization.", d.Id))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if level[result[i].OrgId] != level[result[j].OrgId] {
			return level[result[i].OrgId] < level[result[j].OrgId]
		}
		return result[i].OrgId < result[j].OrgId
	})
	return result
}

// initializeIfEnabled initializes a new sub org when the organization access of its primary org
// enables it. An org that the selection does not cover is initialized when it enters the selection.
func initializeIfEnabled(ctx context.Context, org model.Organization) {

	enabled, err := sharingService.IsOrgEnabled(ctx, org)
	if err != nil {
		log.GetLogger().Warn(fmt.Sprintf("Failed to read the enablement of organization '%s'.", org.OrgHandle),
			log.Error(err))
		return
	}
	if enabled {
		initializeOrg(ctx, org)
	}
}

// initializeOrg initializes a new sub org: it seeds the mandatory consent category. The sub org
// inherits the identity attributes of its primary org. A failure is logged, and the org list shows
// it as initial_sync_done=false.
func initializeOrg(ctx context.Context, org model.Organization) {

	logger := log.GetLogger()
	if err := consentService.GetConsentCategoryService().SeedDefaultConsentCategory(ctx, org.OrgHandle); err != nil {
		logger.Warn(fmt.Sprintf("Failed to seed the consent category of organization '%s'.", org.OrgHandle),
			log.Error(err))
		return
	}
	if err := adminConfigStore.UpdateInitialSchemaSyncConfig(ctx, true, org.OrgHandle); err != nil {
		logger.Warn(fmt.Sprintf("Failed to mark the initial sync of organization '%s'.", org.OrgHandle),
			log.Error(err))
	}
}

// HandleSyncEvent applies an org lifecycle event that the identity provider pushes. pathHandle is
// the org of the request path, which is an org of the same tree. CDS ignores the events of a
// primary org until its first tree sync starts.
func HandleSyncEvent(ctx context.Context, pathHandle string, event model.SyncEvent) error {

	switch event.Event {
	case model.EventOrgCreated, model.EventOrgUpdated, model.EventOrgDeleted:
	default:
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:    errors2.BAD_REQUEST.Code,
			Message: errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Unsupported organization event '%s'. Use %s, %s, or %s.", event.Event,
				model.EventOrgCreated, model.EventOrgUpdated, model.EventOrgDeleted),
		}, http.StatusBadRequest)
	}
	primaryHandle, err := enabledPrimaryHandle(ctx, pathHandle)
	if err != nil {
		return err
	}
	status, err := store.GetTreeSyncStatus(ctx, primaryHandle)
	if err != nil {
		return err
	}
	primary, err := store.GetOrganizationByHandle(ctx, primaryHandle)
	if err != nil {
		return err
	}
	if status.State == model.TreeSyncNotSynced || primary == nil {
		log.GetLogger().Debug(fmt.Sprintf("Ignoring the organization event %s of '%s', because the organizations "+
			"of primary organization '%s' are not synced.", event.Event, event.OrgId, primaryHandle))
		return nil
	}

	switch event.Event {
	case model.EventOrgCreated, model.EventOrgUpdated:
		info, err := idp.GetAdapter().GetOrganization(ctx, primaryHandle, event.OrgId)
		if err != nil {
			return err
		}
		if info == nil {
			log.GetLogger().Info(fmt.Sprintf("The identity provider does not report organization '%s' under "+
				"primary organization '%s'. Ignoring the event.", event.OrgId, primaryHandle))
			return nil
		}
		parent, err := store.GetOrganizationById(ctx, info.ParentId)
		if err != nil {
			return err
		}
		previous, err := store.GetOrganizationById(ctx, info.Id)
		if err != nil {
			return err
		}
		// An unknown parent or a move needs the full tree. The sync runs in the background.
		if parent == nil || (previous != nil && previous.ParentOrgId != info.ParentId) {
			return scheduleTreeSync(ctx, primaryHandle)
		}
		status := model.StatusActive
		if info.Status == model.StatusDisabled {
			status = model.StatusDisabled
		}
		org := model.Organization{
			OrgId:        info.Id,
			OrgHandle:    info.Handle,
			OrgName:      info.Name,
			ParentOrgId:  parent.OrgId,
			PrimaryOrgId: primary.OrgId,
			Status:       status,
		}
		if err := store.UpsertOrganization(ctx, org); err != nil {
			return err
		}
		if previous == nil || previous.Status == model.StatusDeleted {
			initializeIfEnabled(ctx, org)
		}
		return nil

	case model.EventOrgDeleted:
		previous, err := store.GetOrganizationById(ctx, event.OrgId)
		if err != nil || previous == nil {
			return err
		}
		return store.UpdateOrganizationStatus(ctx, previous.OrgId, model.StatusDeleted)
	}
	return nil
}

// enabledPrimaryHandle returns the handle of the primary org of the org, when the primary org
// enabled CDS.
func enabledPrimaryHandle(ctx context.Context, orgHandle string) (string, error) {

	primaryHandle := orgHandle
	org, err := store.GetOrganizationByHandle(ctx, orgHandle)
	if err != nil {
		return "", err
	}
	if org != nil && !org.IsPrimary() {
		primary, err := store.GetOrganizationById(ctx, org.PrimaryOrgId)
		if err != nil {
			return "", err
		}
		if primary != nil {
			primaryHandle = primary.OrgHandle
		}
	}
	config, err := adminConfigStore.GetAdminConfig(ctx, primaryHandle)
	if err != nil {
		return "", err
	}
	if config == nil || !config.CDSEnabled {
		return "", errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CDS_NOT_ENABLED.Code,
			Message:     errors2.CDS_NOT_ENABLED.Message,
			Description: errors2.CDS_NOT_ENABLED.Description,
		}, http.StatusBadRequest)
	}
	return primaryHandle, nil
}

// PrimaryHandleOf returns the handle of the primary org of the org. The bool is false when CDS does
// not know the org, or when the sub org is not active.
func PrimaryHandleOf(ctx context.Context, orgHandle string) (string, bool) {

	org, err := store.GetOrganizationByHandle(ctx, orgHandle)
	if err != nil || org == nil {
		return "", false
	}
	if org.IsPrimary() {
		return org.OrgHandle, true
	}
	if org.Status != model.StatusActive {
		return "", false
	}
	primary, err := store.GetOrganizationById(ctx, org.PrimaryOrgId)
	if err != nil || primary == nil {
		return "", false
	}
	return primary.OrgHandle, true
}

// SubOrgHandleOf returns the handle of the sub org of the path /t/{primary_handle}/o/{org_id}/...
// The org must be a sub org in the tree of the primary org. An org that CDS does not know is not
// found.
func SubOrgHandleOf(ctx context.Context, primaryHandle, orgId string) (string, error) {

	org, err := store.GetOrganizationById(ctx, orgId)
	if err != nil {
		return "", err
	}
	notFound := fmt.Errorf("the organization '%s' is not in the organization tree of '%s'", orgId, primaryHandle)
	if org == nil || org.IsPrimary() || org.Status == model.StatusDeleted {
		return "", notFound
	}
	primary, err := store.GetOrganizationById(ctx, org.PrimaryOrgId)
	if err != nil {
		return "", err
	}
	if primary == nil || primary.OrgHandle != primaryHandle {
		return "", notFound
	}
	return org.OrgHandle, nil
}

// IsSubOrgHandle reports whether CDS knows the handle as a sub org. A handle that CDS does not know
// is not a sub org.
func IsSubOrgHandle(ctx context.Context, orgHandle string) bool {

	org, err := store.GetOrganizationByHandle(ctx, orgHandle)
	return err == nil && org != nil && !org.IsPrimary()
}
