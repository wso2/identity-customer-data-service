/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
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

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

var SchemaSyncQueue chan model.ProfileSchemaSync
var startOnce sync.Once

const defaultQueueSize = 1000

// StartSchemaSyncWorker initializes and starts the schema sync worker
// This function can be called multiple times safely; it will only initialize once
func StartSchemaSyncWorker() {

	startOnce.Do(func() {
		// Initialize the queue with a buffer size (configurable in future via config)
		SchemaSyncQueue = make(chan model.ProfileSchemaSync, defaultQueueSize)

		go func() {
			for schemaSync := range SchemaSyncQueue {
				processSchemaSyncJob(schemaSync)
			}
		}()
	})
}

// EnqueueSchemaSyncJob adds a schema sync job to the queue
// Returns true if successfully enqueued, false if queue is full
func EnqueueSchemaSyncJob(schemaSync model.ProfileSchemaSync) bool {
	if SchemaSyncQueue == nil {
		log.GetLogger().Error("Schema sync queue is not initialized. Cannot enqueue job.")
		return false
	}
	
	// Use non-blocking send to avoid hanging if queue is full
	select {
	case SchemaSyncQueue <- schemaSync:
		return true
	default:
		log.GetLogger().Error(fmt.Sprintf("Schema sync queue is full. Cannot enqueue job for tenant: %s", schemaSync.OrgId))
		return false
	}
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
