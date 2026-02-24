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

// Package inmemory provides in-memory (buffered channel) implementations of
// the queue.ProfileUnificationQueue and queue.SchemaSyncQueue interfaces.
// This is the default provider and is suitable for single-instance, local,
// and development deployments.
package inmemory

import (
	"fmt"

	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// ProfileQueue is the in-memory implementation of queue.ProfileUnificationQueue.
// It uses a buffered Go channel as the underlying queue.
type ProfileQueue struct {
	ch chan profileModel.Profile
}

// NewProfileQueue creates a new ProfileQueue with the given buffer size.
func NewProfileQueue(size int) *ProfileQueue {
	return &ProfileQueue{ch: make(chan profileModel.Profile, size)}
}

// Enqueue adds a profile to the in-memory channel. It is non-blocking: if
// the channel is full the item is dropped and false is returned.
func (q *ProfileQueue) Enqueue(profile profileModel.Profile) bool {
	select {
	case q.ch <- profile:
		return true
	default:
		log.GetLogger().Error(fmt.Sprintf(
			"In-memory profile unification queue is full. Dropping profile: %s", profile.ProfileId))
		return false
	}
}

// Start launches a goroutine that reads profiles from the channel and
// forwards each one to handler. The goroutine runs until the channel is
// closed. Always returns nil.
func (q *ProfileQueue) Start(handler func(profileModel.Profile)) error {
	go func() {
		for profile := range q.ch {
			handler(profile)
		}
	}()
	return nil
}

// SchemaSyncQueue is the in-memory implementation of queue.SchemaSyncQueue.
// It uses a buffered Go channel as the underlying queue.
type SchemaSyncQueue struct {
	ch chan schemaModel.ProfileSchemaSync
}

// NewSchemaSyncQueue creates a new SchemaSyncQueue with the given buffer size.
func NewSchemaSyncQueue(size int) *SchemaSyncQueue {
	return &SchemaSyncQueue{ch: make(chan schemaModel.ProfileSchemaSync, size)}
}

// Enqueue adds a schema sync job to the in-memory channel. It is non-blocking:
// if the channel is full the item is dropped and false is returned.
func (q *SchemaSyncQueue) Enqueue(sync schemaModel.ProfileSchemaSync) bool {
	select {
	case q.ch <- sync:
		return true
	default:
		log.GetLogger().Error(fmt.Sprintf(
			"In-memory schema sync queue is full. Dropping job for tenant: %s", sync.OrgId))
		return false
	}
}

// Start launches a goroutine that reads schema sync jobs from the channel and
// forwards each one to handler. The goroutine runs until the channel is
// closed. Always returns nil.
func (q *SchemaSyncQueue) Start(handler func(schemaModel.ProfileSchemaSync)) error {
	go func() {
		for sync := range q.ch {
			handler(sync)
		}
	}()
	return nil
}
