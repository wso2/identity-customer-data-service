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

// Package queue provides a pluggable message queue abstraction for profile
// unification and schema synchronization. The default provider is an
// in-memory channel-based queue suitable for lightweight and local
// deployments. An ActiveMQ provider is also available for production
// workloads that require durability across restarts and scaling events.
package queue

import (
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
)

// Provider type constants used in configuration.
const (
	TypeMemory   = "memory"
	TypeActiveMQ = "activemq"
)

// ProfileUnificationQueue defines the contract for enqueuing profiles for
// asynchronous unification processing.
type ProfileUnificationQueue interface {
	// Enqueue adds a profile to the queue for unification. It returns true
	// when the item is accepted and false when the queue is full or
	// unavailable.
	Enqueue(profile profileModel.Profile) bool

	// Start begins consuming queue items and invokes handler for each one.
	// Implementations must start the consumer loop in a separate goroutine
	// so that Start returns immediately. An error is returned when the queue
	// cannot be started (e.g. broker subscription failure).
	Start(handler func(profileModel.Profile)) error
}

// SchemaSyncQueue defines the contract for enqueuing schema synchronisation
// jobs for asynchronous processing.
type SchemaSyncQueue interface {
	// Enqueue adds a schema sync job to the queue. It returns true when the
	// item is accepted and false when the queue is full or unavailable.
	Enqueue(sync schemaModel.ProfileSchemaSync) bool

	// Start begins consuming queue items and invokes handler for each one.
	// Implementations must start the consumer loop in a separate goroutine
	// so that Start returns immediately. An error is returned when the queue
	// cannot be started (e.g. broker subscription failure).
	Start(handler func(schemaModel.ProfileSchemaSync)) error
}
