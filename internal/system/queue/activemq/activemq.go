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

// Package activemq provides ActiveMQ-backed implementations of the
// queue.ProfileUnificationQueue and queue.SchemaSyncQueue interfaces using
// the STOMP protocol. This provider is intended for production deployments
// that require durable messaging across restarts and horizontal scaling.
//
// To activate this provider, blank-import this package in main.go (or any
// other entry-point) so that its init() function registers it:
//
//	import _ "github.com/wso2/identity-customer-data-service/internal/system/queue/activemq"
//
// Then set message_queue.type = "activemq" in deployment.yaml.
package activemq

import (
	"encoding/json"
	"fmt"

	"github.com/go-stomp/stomp/v3"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
)

const contentTypeJSON = "application/json"

// init registers the ActiveMQ provider with the queue factory so it is
// available as soon as this package is imported.
func init() {
	queue.RegisterProfileQueueProvider(queue.TypeActiveMQ,
		func(cfg config.ExternalBrokerConfig) (queue.ProfileUnificationQueue, error) {
			return NewProfileQueue(cfg.Addr, cfg.Username, cfg.Password, cfg.ProfileQueueName)
		},
	)
	queue.RegisterSchemaSyncQueueProvider(queue.TypeActiveMQ,
		func(cfg config.ExternalBrokerConfig) (queue.SchemaSyncQueue, error) {
			return NewSchemaSyncQueue(cfg.Addr, cfg.Username, cfg.Password, cfg.SchemaSyncQueueName)
		},
	)
}

// ProfileQueue is the ActiveMQ-backed implementation of
// queue.ProfileUnificationQueue. It communicates with ActiveMQ over the
// STOMP protocol.
type ProfileQueue struct {
	conn        *stomp.Conn
	destination string
}

// NewProfileQueue dials ActiveMQ at addr (host:port) using the supplied
// credentials and returns a ProfileQueue that publishes to / consumes from
// destination.
func NewProfileQueue(addr, username, password, destination string) (*ProfileQueue, error) {
	conn, err := stomp.Dial("tcp", addr,
		stomp.ConnOpt.Login(username, password),
	)
	if err != nil {
		return nil, fmt.Errorf("activemq: failed to connect for profile queue: %w", err)
	}
	return &ProfileQueue{conn: conn, destination: destination}, nil
}

// Enqueue serialises profile to JSON and sends it to the ActiveMQ
// destination. Returns false on serialisation or send failure.
func (q *ProfileQueue) Enqueue(profile profileModel.Profile) bool {
	data, err := json.Marshal(profile)
	if err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: failed to marshal profile %s: %v", profile.ProfileId, err))
		return false
	}
	if err := q.conn.Send(q.destination, contentTypeJSON, data); err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: failed to send profile %s to queue: %v", profile.ProfileId, err))
		return false
	}
	return true
}

// Start subscribes to the ActiveMQ destination and launches a goroutine that
// deserialises each incoming message and passes it to handler. Returns an
// error when the subscription cannot be established.
func (q *ProfileQueue) Start(handler func(profileModel.Profile)) error {
	sub, err := q.conn.Subscribe(q.destination, stomp.AckAuto)
	if err != nil {
		return fmt.Errorf("activemq: failed to subscribe to profile queue %s: %w", q.destination, err)
	}
	go func() {
		for msg := range sub.C {
			if msg.Err != nil {
				log.GetLogger().Error(fmt.Sprintf(
					"activemq: error receiving profile message: %v", msg.Err))
				continue
			}
			var profile profileModel.Profile
			if err := json.Unmarshal(msg.Body, &profile); err != nil {
				log.GetLogger().Error(fmt.Sprintf(
					"activemq: failed to unmarshal profile message: %v", err))
				continue
			}
			handler(profile)
		}
	}()
	return nil
}

// SchemaSyncQueue is the ActiveMQ-backed implementation of
// queue.SchemaSyncQueue. It communicates with ActiveMQ over the STOMP
// protocol.
type SchemaSyncQueue struct {
	conn        *stomp.Conn
	destination string
}

// NewSchemaSyncQueue dials ActiveMQ at addr (host:port) using the supplied
// credentials and returns a SchemaSyncQueue that publishes to / consumes
// from destination.
func NewSchemaSyncQueue(addr, username, password, destination string) (*SchemaSyncQueue, error) {
	conn, err := stomp.Dial("tcp", addr,
		stomp.ConnOpt.Login(username, password),
	)
	if err != nil {
		return nil, fmt.Errorf("activemq: failed to connect for schema sync queue: %w", err)
	}
	return &SchemaSyncQueue{conn: conn, destination: destination}, nil
}

// Enqueue serialises sync to JSON and sends it to the ActiveMQ destination.
// Returns false on serialisation or send failure.
func (q *SchemaSyncQueue) Enqueue(sync schemaModel.ProfileSchemaSync) bool {
	data, err := json.Marshal(sync)
	if err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: failed to marshal schema sync for tenant %s: %v", sync.OrgId, err))
		return false
	}
	if err := q.conn.Send(q.destination, contentTypeJSON, data); err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: failed to send schema sync for tenant %s to queue: %v", sync.OrgId, err))
		return false
	}
	return true
}

// Start subscribes to the ActiveMQ destination and launches a goroutine that
// deserialises each incoming message and passes it to handler. Returns an
// error when the subscription cannot be established.
func (q *SchemaSyncQueue) Start(handler func(schemaModel.ProfileSchemaSync)) error {
	sub, err := q.conn.Subscribe(q.destination, stomp.AckAuto)
	if err != nil {
		return fmt.Errorf("activemq: failed to subscribe to schema sync queue %s: %w", q.destination, err)
	}
	go func() {
		for msg := range sub.C {
			if msg.Err != nil {
				log.GetLogger().Error(fmt.Sprintf(
					"activemq: error receiving schema sync message: %v", msg.Err))
				continue
			}
			var sync schemaModel.ProfileSchemaSync
			if err := json.Unmarshal(msg.Body, &sync); err != nil {
				log.GetLogger().Error(fmt.Sprintf(
					"activemq: failed to unmarshal schema sync message: %v", err))
				continue
			}
			handler(sync)
		}
	}()
	return nil
}
