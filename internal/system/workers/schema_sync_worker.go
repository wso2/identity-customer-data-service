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

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
)

// activeSchemaSyncQueue is the queue implementation used for schema
// synchronisation. It is initialised by StartSchemaSyncWorker.
var activeSchemaSyncQueue queue.SchemaSyncQueue

// StartSchemaSyncWorker initialises the schema sync queue (using the provider
// configured in the runtime config) and starts the consumer goroutine. An
// error is returned when the queue cannot be created or started; the caller
// should treat this as a fatal startup failure.
func StartSchemaSyncWorker() error {
	cfg := config.GetCDSRuntime().Config.MessageQueue
	q, err := queue.NewSchemaSyncQueue(cfg)
	if err != nil {
		return fmt.Errorf("workers: failed to create schema sync queue: %w", err)
	}
	activeSchemaSyncQueue = q
	if err := q.Start(processSchemaSyncJob); err != nil {
		return fmt.Errorf("workers: failed to start schema sync queue: %w", err)
	}
	return nil
}

// EnqueueSchemaSyncJob adds a schema sync job to the active queue. It
// returns false when the worker has not been started or the queue is full.
func EnqueueSchemaSyncJob(schemaSync model.ProfileSchemaSync) bool {
	if activeSchemaSyncQueue == nil {
		log.GetLogger().Error("Schema sync queue is not initialized. Cannot enqueue job.")
		return false
	}
	return activeSchemaSyncQueue.Enqueue(schemaSync)
}

// processSchemaSyncJob processes a schema sync job
func processSchemaSyncJob(schemaSync model.ProfileSchemaSync) {

	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Processing schema sync job for tenant: %s, event: %s", schemaSync.OrgId, schemaSync.Event))

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()

	err := schemaService.SyncProfileSchema(schemaSync.OrgId)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to sync profile schema for tenant: %s", schemaSync.OrgId), log.Error(err))
		return
	}

	logger.Info(fmt.Sprintf("Profile schema sync completed successfully for tenant: %s", schemaSync.OrgId))
}
