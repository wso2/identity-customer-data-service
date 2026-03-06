/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
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
	"fmt"
	"sync"

	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
)

var (
	profileDataMigrationQueueMu     sync.RWMutex
	activeProfileDataMigrationQueue queue.ProfileDataMigrationQueue
)

// StartProfileDataMigrationWorker initialises the profile data migration queue
// and starts the consumer goroutine.  An error is returned when the queue
// cannot be created or started; the caller should treat this as a fatal
// startup failure.
func StartProfileDataMigrationWorker() error {
	cfg := config.GetCDSRuntime().Config.MessageQueue
	q, err := queue.NewProfileDataMigrationQueue(cfg)
	if err != nil {
		return fmt.Errorf("workers: failed to create profile data migration queue: %w", err)
	}
	if err := q.Start(processProfileDataMigrationJob); err != nil {
		_ = q.Close()
		return fmt.Errorf("workers: failed to start profile data migration queue: %w", err)
	}
	profileDataMigrationQueueMu.Lock()
	activeProfileDataMigrationQueue = q
	profileDataMigrationQueueMu.Unlock()
	return nil
}

// EnqueueProfileDataMigrationJob adds a migration job to the active queue.
// Enqueue errors are logged but not propagated because profile data cleanup
// is a best-effort background task; the schema update has already committed.
func EnqueueProfileDataMigrationJob(job schemaModel.SchemaChangeJob) error {
	profileDataMigrationQueueMu.RLock()
	q := activeProfileDataMigrationQueue
	profileDataMigrationQueueMu.RUnlock()
	if q == nil {
		return fmt.Errorf("profile data migration queue is not initialized")
	}
	return q.Enqueue(job)
}

// StopProfileDataMigrationWorker gracefully shuts down the queue.
func StopProfileDataMigrationWorker() error {
	profileDataMigrationQueueMu.Lock()
	q := activeProfileDataMigrationQueue
	activeProfileDataMigrationQueue = nil
	profileDataMigrationQueueMu.Unlock()
	if q != nil {
		return q.Close()
	}
	return nil
}

// processProfileDataMigrationJob handles a single migration job by delegating
// to the appropriate profile store operation based on the change type.
func processProfileDataMigrationJob(job schemaModel.SchemaChangeJob) {
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf(
		"Processing profile data migration: org=%s scope=%s key=%v change=%s",
		job.OrgId, job.Scope, job.KeyPath, job.ChangeType,
	))

	var err error
	switch job.ChangeType {
	case schemaModel.ChangeTypeDeleted:
		err = profileStore.RemoveProfileAttribute(job.OrgId, job.Scope, job.KeyPath, job.AppId)
	case schemaModel.ChangeTypeTypeChanged:
		err = profileStore.NullifyProfileAttribute(job.OrgId, job.Scope, job.KeyPath, job.AppId)
	default:
		logger.Warn(fmt.Sprintf("Unknown schema change type %q, skipping job", job.ChangeType))
		return
	}

	if err != nil {
		logger.Error(fmt.Sprintf(
			"Profile data migration failed: org=%s scope=%s key=%v change=%s: %v",
			job.OrgId, job.Scope, job.KeyPath, job.ChangeType, err,
		))
		return
	}
	logger.Info(fmt.Sprintf(
		"Profile data migration completed: org=%s scope=%s key=%v change=%s",
		job.OrgId, job.Scope, job.KeyPath, job.ChangeType,
	))
}
