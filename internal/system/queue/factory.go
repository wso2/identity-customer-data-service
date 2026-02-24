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

package queue

import (
	"fmt"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/queue/activemq"
	"github.com/wso2/identity-customer-data-service/internal/system/queue/inmemory"
)

// NewProfileUnificationQueue creates the ProfileUnificationQueue implementation
// specified by cfg.Type. When the type is empty or "memory" the default
// in-memory provider is returned. When the type is "activemq" an ActiveMQ
// provider is returned using the settings in cfg.ActiveMQ.
func NewProfileUnificationQueue(cfg config.MessageQueueConfig) (ProfileUnificationQueue, error) {
	switch cfg.Type {
	case TypeActiveMQ:
		q, err := activemq.NewProfileQueue(
			cfg.ActiveMQ.Addr,
			cfg.ActiveMQ.Username,
			cfg.ActiveMQ.Password,
			cfg.ActiveMQ.ProfileQueueName,
		)
		if err != nil {
			return nil, fmt.Errorf("queue: failed to create ActiveMQ profile queue: %w", err)
		}
		return q, nil
	default:
		// TypeMemory or any unrecognised value – fall back to in-memory.
		return inmemory.NewProfileQueue(constants.DefaultQueueSize), nil
	}
}

// NewSchemaSyncQueue creates the SchemaSyncQueue implementation specified by
// cfg.Type. When the type is empty or "memory" the default in-memory provider
// is returned. When the type is "activemq" an ActiveMQ provider is returned
// using the settings in cfg.ActiveMQ.
func NewSchemaSyncQueue(cfg config.MessageQueueConfig) (SchemaSyncQueue, error) {
	switch cfg.Type {
	case TypeActiveMQ:
		q, err := activemq.NewSchemaSyncQueue(
			cfg.ActiveMQ.Addr,
			cfg.ActiveMQ.Username,
			cfg.ActiveMQ.Password,
			cfg.ActiveMQ.SchemaSyncQueueName,
		)
		if err != nil {
			return nil, fmt.Errorf("queue: failed to create ActiveMQ schema sync queue: %w", err)
		}
		return q, nil
	default:
		// TypeMemory or any unrecognised value – fall back to in-memory.
		return inmemory.NewSchemaSyncQueue(constants.DefaultQueueSize), nil
	}
}
