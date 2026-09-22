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
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-stomp/stomp/v3"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
)

const (
	// Three total processing attempts, with 2s and 4s delays before retries.
	// Only errors explicitly marked safe to retry consume this allowance.
	maxProcessingAttempts = 3
	attemptHeader         = "cds-processing-attempt"
	sourceHeader          = "cds-original-destination"
	messageIDHeader       = "cds-original-message-id"
	failureHeader         = "cds-failure-category"
)

// boundedWriteConn bounds socket writes even when the broker stops reading.
// Receipt and channel timeouts alone do not interrupt a blocked network write.
type boundedWriteConn struct{ net.Conn }

func (c *boundedWriteConn) Write(p []byte) (int, error) {
	if err := c.Conn.SetWriteDeadline(time.Now().Add(brokerIOTimeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}

func startConsumer(mc *managedConn, destination string, handler func([]byte) error) error {
	// This provider uses point-to-point queues, including a sibling DLQ.
	if !strings.HasPrefix(destination, "/queue/") || len(destination) == len("/queue/") {
		return fmt.Errorf("activemq: destination must be a named /queue/ destination")
	}
	sub, generation, err := mc.subscribeCurrent(destination)
	if err != nil {
		return err
	}
	go mc.consume(destination, sub, generation, handler)
	return nil
}

func (mc *managedConn) consume(destination string, sub *stomp.Subscription, generation uint64,
	handler func([]byte) error) {
	for {
		select {
		case <-mc.done:
			return
		case msg, ok := <-sub.C:
			if ok && msg != nil && msg.Err == nil {
				err := mc.processMessage(destination, msg, handler)
				if err == nil {
					continue
				}
				if errors.Is(err, queue.ErrDeferred) {
					// The worker rejected new work during drain. Close follows
					// drain; leave this delivery on the broker until then.
					<-mc.done
					return
				}
				log.GetLogger().Error("activemq: settlement failed; retaining unacknowledged work and reconnecting")
			}
		}
		var err error
		sub, generation, err = mc.recoverSubscription(destination, generation)
		if err != nil {
			return
		}
	}
}

func (mc *managedConn) recoverSubscription(destination string, generation uint64) (*stomp.Subscription, uint64, error) {
	for !mc.isShuttingDown() {
		_, current := mc.getConnAndGeneration()
		if current == generation {
			if err := mc.reconnectWithBackoff("consumer", 0); err != nil {
				if mc.isShuttingDown() {
					return nil, generation, err
				}
				continue
			}
		}
		sub, next, err := mc.subscribeCurrent(destination)
		if err == nil {
			return sub, next, nil
		}
		// A failed subscribe on the new generation must trigger a backoff
		// and reconnect, not spin on the old subscription's closed channel.
		generation = next
	}
	return nil, generation, queue.ErrDeferred
}

func (mc *managedConn) processMessage(destination string, msg *stomp.Message, handler func([]byte) error) error {
	if mc.isShuttingDown() {
		return queue.ErrDeferred
	}
	attempt := 1
	var processingErr error
	if value := msg.Header.Get(attemptHeader); value != "" {
		attempt, processingErr = strconv.Atoi(value)
		if processingErr != nil || attempt < 1 || attempt > maxProcessingAttempts {
			processingErr = errors.New("invalid processing attempt")
		}
	}
	if processingErr == nil && attempt > 1 {
		timer := time.NewTimer(initialBackoff * time.Duration(1<<(attempt-2)))
		select {
		case <-mc.done:
			timer.Stop()
			return queue.ErrDeferred
		case <-timer.C:
		}
	}
	if processingErr == nil {
		processingErr = handler(msg.Body)
	}
	if mc.isShuttingDown() || errors.Is(processingErr, queue.ErrDeferred) {
		return queue.ErrDeferred
	}
	if processingErr == nil {
		return msg.Conn.Ack(msg)
	}

	target, category := destination+".DLQ", "terminal"
	nextAttempt := attempt
	if queue.IsRetryable(processingErr) {
		category = "exhausted"
		if attempt < maxProcessingAttempts {
			target, category, nextAttempt = destination, "retry", attempt+1
		}
	}
	if err := mc.transferMessage(msg, destination, target, category, nextAttempt); err != nil {
		return err
	}
	// Do not log message bodies or raw errors: they can contain profile data,
	// HTTP responses or broker credentials. Correlation is kept on the message.
	log.GetLogger().Warn(fmt.Sprintf("activemq: processing outcome=%s attempt=%d", category, attempt))
	return nil
}

// transferMessage atomically publishes the retry/DLQ copy and acknowledges
// its source. A lost commit receipt leaves the outcome uncertain, but the
// broker commits both operations or neither; never acknowledge separately.
func (mc *managedConn) transferMessage(msg *stomp.Message, source, target, category string, attempt int) error {
	mc.mu.RLock()
	conn, socket := mc.conn, mc.netConn
	mc.mu.RUnlock()
	if conn != msg.Conn {
		return fmt.Errorf("activemq: delivery belongs to a retired connection")
	}
	// go-stomp's transaction receipt wait does not use RcvReceiptTimeout
	// when heartbeats are disabled. Close the socket to bound the whole
	// transfer and release the library's receipt waiter.
	timer := time.AfterFunc(brokerIOTimeout, func() { _ = socket.Close() })
	defer timer.Stop()
	tx, err := msg.Conn.BeginWithError()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Abort() }()
	originalID := msg.Header.Get(messageIDHeader)
	if originalID == "" {
		originalID = msg.Header.Get("message-id")
	}
	err = tx.Send(target, msg.ContentType, msg.Body,
		stomp.SendOpt.Header("persistent", "true"),
		stomp.SendOpt.Header(attemptHeader, strconv.Itoa(attempt)),
		stomp.SendOpt.Header(sourceHeader, source),
		stomp.SendOpt.Header(messageIDHeader, originalID),
		stomp.SendOpt.Header(failureHeader, category))
	if err != nil {
		return err
	}
	if err := tx.Ack(msg); err != nil {
		return err
	}
	return tx.CommitWithReceipt()
}
