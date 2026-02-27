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

package activemq

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-stomp/stomp/v3"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
)

const (
	contentTypeJSON   = "application/json"
	initialBackoff    = 2 * time.Second
	maxBackoff        = 60 * time.Second
	backoffMultiplier = 2
)

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

// managedConn holds a STOMP connection and re-dials transparently when the
// connection is lost. All reconnect attempts use exponential backoff capped
// at maxBackoff.
type managedConn struct {
	addr     string
	username string
	password string
	conn     *stomp.Conn
}

func newManagedConn(addr, username, password string) (*managedConn, error) {
	mc := &managedConn{addr: addr, username: username, password: password}
	if err := mc.dial(); err != nil {
		return nil, err
	}
	return mc, nil
}

func (mc *managedConn) dial() error {
	conn, err := stomp.Dial("tcp", mc.addr, stomp.ConnOpt.Login(mc.username, mc.password))
	if err != nil {
		return fmt.Errorf("activemq: dial %s: %w", mc.addr, err)
	}
	mc.conn = conn
	return nil
}

// reconnectWithBackoff blocks until a connection is re-established, logging
// each failed attempt. It is called whenever a send or subscribe detects that
// the underlying connection is dead.
func (mc *managedConn) reconnectWithBackoff(context string) {
	logger := log.GetLogger()
	backoff := initialBackoff
	for {
		logger.Error(fmt.Sprintf("activemq: connection lost (%s), reconnecting in %s…", context, backoff))
		time.Sleep(backoff)
		if err := mc.dial(); err != nil {
			logger.Error(fmt.Sprintf("activemq: reconnect failed: %v", err))
			backoff *= backoffMultiplier
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		logger.Info(fmt.Sprintf("activemq: reconnected successfully (%s)", context))
		return
	}
}

// -----------------------------------------------------------------------
// ProfileQueue
// -----------------------------------------------------------------------

// ProfileQueue is the ActiveMQ-backed ProfileUnificationQueue.
type ProfileQueue struct {
	mc          *managedConn
	destination string
}

func NewProfileQueue(addr, username, password, destination string) (*ProfileQueue, error) {
	mc, err := newManagedConn(addr, username, password)
	if err != nil {
		return nil, fmt.Errorf("activemq: failed to connect for profile queue: %w", err)
	}
	return &ProfileQueue{mc: mc, destination: destination}, nil
}

// Enqueue marshals the profile to JSON and sends it to ActiveMQ.
//
// Retry policy: if the initial send fails (typically a dead connection),
// Enqueue reconnects once with backoff and retries a single time. This is
// intentionally limited compared to the infinite-retry loop in Start,
// because Enqueue is called in the request path where blocking
// indefinitely is unacceptable. Callers that need stronger delivery
// guarantees should persist the item and retry externally.
func (q *ProfileQueue) Enqueue(profile profileModel.Profile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("activemq: failed to marshal profile %s: %w", profile.ProfileId, err)
	}

	if err := q.mc.conn.Send(q.destination, contentTypeJSON, data); err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: send failed for profile %s, will reconnect and retry: %v",
			profile.ProfileId, err))
		q.mc.reconnectWithBackoff("profile enqueue")

		// Single retry after reconnect.
		if retryErr := q.mc.conn.Send(q.destination, contentTypeJSON, data); retryErr != nil {
			return fmt.Errorf("activemq: retry send failed for profile %s: %w", profile.ProfileId, retryErr)
		}
	}
	return nil
}

// Start subscribes to the destination and launches a consumer goroutine.
//
// Retry policy: the consumer loop retries forever with exponential backoff
// when the subscription is lost, because the consumer is a long-lived
// background goroutine that must stay alive for the lifetime of the
// process. This is intentionally different from the single-retry in
// Enqueue (see Enqueue docs for rationale).
func (q *ProfileQueue) Start(handler func(profileModel.Profile)) error {
	sub, err := q.mc.conn.Subscribe(q.destination, stomp.AckAuto)
	if err != nil {
		return fmt.Errorf("activemq: failed to subscribe to profile queue %s: %w", q.destination, err)
	}

	go func() {
		for {
			msg, ok := <-sub.C
			if !ok {
				// Channel closed — connection dropped. Reconnect and re-subscribe.
				log.GetLogger().Error("activemq: profile queue subscription channel closed, reconnecting…")
				q.mc.reconnectWithBackoff("profile consumer")
				newSub, err := q.mc.conn.Subscribe(q.destination, stomp.AckAuto)
				if err != nil {
					log.GetLogger().Error(fmt.Sprintf(
						"activemq: failed to re-subscribe to profile queue: %v", err))
					continue
				}
				sub = newSub
				continue
			}

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

// Close gracefully disconnects from ActiveMQ.
func (q *ProfileQueue) Close() error {
	return q.mc.conn.Disconnect()
}

// -----------------------------------------------------------------------
// SchemaSyncQueue
// -----------------------------------------------------------------------

// SchemaSyncQueue is the ActiveMQ-backed SchemaSyncQueue.
type SchemaSyncQueue struct {
	mc          *managedConn
	destination string
}

func NewSchemaSyncQueue(addr, username, password, destination string) (*SchemaSyncQueue, error) {
	mc, err := newManagedConn(addr, username, password)
	if err != nil {
		return nil, fmt.Errorf("activemq: failed to connect for schema sync queue: %w", err)
	}
	return &SchemaSyncQueue{mc: mc, destination: destination}, nil
}

// Enqueue marshals the schema sync to JSON and sends it to ActiveMQ.
// See ProfileQueue.Enqueue for retry-policy rationale.
func (q *SchemaSyncQueue) Enqueue(sync schemaModel.ProfileSchemaSync) error {
	data, err := json.Marshal(sync)
	if err != nil {
		return fmt.Errorf("activemq: failed to marshal schema sync for tenant %s: %w", sync.OrgId, err)
	}

	if err := q.mc.conn.Send(q.destination, contentTypeJSON, data); err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: send failed for schema sync tenant %s, will reconnect and retry: %v",
			sync.OrgId, err))
		q.mc.reconnectWithBackoff("schema sync enqueue")

		if retryErr := q.mc.conn.Send(q.destination, contentTypeJSON, data); retryErr != nil {
			return fmt.Errorf("activemq: retry send failed for schema sync tenant %s: %w", sync.OrgId, retryErr)
		}
	}
	return nil
}

// Start subscribes to the destination and launches a consumer goroutine.
// See ProfileQueue.Start for retry-policy rationale.
func (q *SchemaSyncQueue) Start(handler func(schemaModel.ProfileSchemaSync)) error {
	sub, err := q.mc.conn.Subscribe(q.destination, stomp.AckAuto)
	if err != nil {
		return fmt.Errorf("activemq: failed to subscribe to schema sync queue %s: %w", q.destination, err)
	}

	go func() {
		for {
			msg, ok := <-sub.C
			if !ok {
				log.GetLogger().Error("activemq: schema sync subscription channel closed, reconnecting…")
				q.mc.reconnectWithBackoff("schema sync consumer")
				newSub, err := q.mc.conn.Subscribe(q.destination, stomp.AckAuto)
				if err != nil {
					log.GetLogger().Error(fmt.Sprintf(
						"activemq: failed to re-subscribe to schema sync queue: %v", err))
					continue
				}
				sub = newSub
				continue
			}

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

// Close gracefully disconnects from ActiveMQ.
func (q *SchemaSyncQueue) Close() error {
	return q.mc.conn.Disconnect()
}
