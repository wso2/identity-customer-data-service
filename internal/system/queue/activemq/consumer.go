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
	"strconv"
	"time"

	"github.com/go-stomp/stomp/v3"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

const (
	// maxDeliveryAttempts bounds how often one message reaches the handler.
	// Without a bound, a message that always fails holds up every message
	// behind it for as long as the deployment runs.
	maxDeliveryAttempts = 5

	// redeliveryDelay separates two attempts at the same message, so that a
	// failure which repeats costs the broker and the handler little.
	redeliveryDelay = 2 * time.Second

	// deadLetterSuffix names the destination that keeps the messages the
	// consumer gave up on. The consumer writes them itself, so the answer
	// does not depend on the redelivery policy of the broker.
	deadLetterSuffix = ".DLQ"

	// Headers that describe why a message is on the dead letter destination.
	headerOriginalDestination = "cds-original-destination"
	headerOriginalMessageID   = "cds-original-message-id"
	headerDeliveryAttempts    = "cds-delivery-attempts"
	headerFailureReason       = "cds-failure-reason"
)

// errMalformed marks a message whose body the handler cannot read. Another
// attempt reads the same bytes, so such a message is set aside at once.
var errMalformed = errors.New("activemq: the message body cannot be read")

// disposition says how the consumer continues after one message.
type disposition int

const (
	// keepGoing means the message has left the queue, or the queue is
	// closing and the close returns the message.
	keepGoing disposition = iota

	// deliverAgain means the message is still in the queue, and the broker
	// must send it once more.
	deliverAgain
)

// consumer reads one destination and tells the broker the outcome of every
// message. A message is acknowledged only after its work is done, so a failed
// job, a lost connection or a stopped process returns the work to the queue.
//
// The work therefore runs at least once, and a job that runs twice must reach
// the same result as a job that runs once.
type consumer struct {
	mc          *managedConn
	destination string
	// name identifies the consumer in the log.
	name string
	// handle runs the work of one message. It returns nil when the work is
	// done, errMalformed when the body cannot be read, and any other error
	// when the work can be tried again.
	handle func(body []byte) error

	// maxAttempts and retryDelay are the delivery policy of this consumer.
	maxAttempts int
	retryDelay  time.Duration

	// attempts counts the deliveries of each message that has already failed,
	// which is how the consumer stops a message that never succeeds. Only the
	// consumer goroutine reads or writes it.
	attempts map[string]int
}

// newConsumer returns a consumer for one destination.
func newConsumer(mc *managedConn, destination, name string, handle func([]byte) error) *consumer {

	return &consumer{
		mc:          mc,
		destination: destination,
		name:        name,
		handle:      handle,
		maxAttempts: maxDeliveryAttempts,
		retryDelay:  redeliveryDelay,
		attempts:    map[string]int{},
	}
}

// start takes the first subscription and reads the destination in its own
// goroutine, so that the caller is not held.
func (c *consumer) start() error {

	sub, generation, err := c.mc.subscribeCurrent(c.destination)
	if err != nil {
		return err
	}

	go c.consume(sub, generation)
	return nil
}

// consume reads one message at a time and keeps the subscription alive for as
// long as the queue is open.
func (c *consumer) consume(sub *stomp.Subscription, generation uint64) {

	for {
		msg, ok := <-sub.C
		if !ok {
			// A subscription whose connection was replaced is simply taken
			// again. A subscription on the live connection means the
			// connection itself is gone.
			_, current := c.mc.getConnAndGeneration()
			next, nextGeneration, running := c.subscribeAgain(generation == current)
			if !running {
				return
			}
			sub, generation = next, nextGeneration
			continue
		}

		if msg.Err != nil {
			log.GetLogger().Error(fmt.Sprintf("activemq: %s consumer received an error: %v", c.name, msg.Err))
			continue
		}

		if c.deliver(msg) == deliverAgain {
			next, nextGeneration, running := c.redeliver(sub)
			if !running {
				return
			}
			sub, generation = next, nextGeneration
		}
	}
}

// deliver runs the work of one message and reports the outcome to the broker.
func (c *consumer) deliver(msg *stomp.Message) disposition {

	id := messageID(msg)

	if err := c.handle(msg.Body); err != nil {
		return c.failed(msg, id, err)
	}

	delete(c.attempts, id)
	if err := c.mc.getConn().Ack(msg); err != nil {
		// The broker still holds the message, so the work runs again.
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: %s message %s was processed but not acknowledged, so it stays in the queue: %v",
			c.name, id, err))
	}
	return keepGoing
}

// failed decides what becomes of a message whose work did not succeed. The
// message stays in the queue until it has had its attempts, and then it goes
// to the dead letter destination so the queue behind it can move.
func (c *consumer) failed(msg *stomp.Message, id string, cause error) disposition {

	logger := log.GetLogger()

	if c.mc.isShuttingDown() {
		// The close of the connection returns the message to the queue, and
		// the instance that takes it next runs the work.
		logger.Info(fmt.Sprintf(
			"activemq: %s message %s stays in the queue because the queue is closing: %v", c.name, id, cause))
		return keepGoing
	}

	c.attempts[id]++
	attempts := c.attempts[id]

	if errors.Is(cause, errMalformed) || attempts >= c.maxAttempts {
		if err := c.setAside(msg, id, attempts, cause); err != nil {
			logger.Error(fmt.Sprintf(
				"activemq: %s message %s could not be moved to %s, so it stays in the queue: %v",
				c.name, id, c.deadLetterDestination(), err))
			return deliverAgain
		}

		delete(c.attempts, id)
		if err := c.mc.getConn().Ack(msg); err != nil {
			logger.Error(fmt.Sprintf(
				"activemq: %s message %s was moved to %s but not acknowledged: %v",
				c.name, id, c.deadLetterDestination(), err))
		}

		logger.Error(fmt.Sprintf(
			"activemq: %s message %s failed %d times and was moved to %s: %v",
			c.name, id, attempts, c.deadLetterDestination(), cause))
		return keepGoing
	}

	logger.Error(fmt.Sprintf("activemq: %s message %s failed on attempt %d of %d: %v",
		c.name, id, attempts, c.maxAttempts, cause))
	return deliverAgain
}

// setAside copies the message to the dead letter destination and waits for the
// broker to confirm it. The original is acknowledged only afterwards, so a
// message is never dropped for a copy that did not arrive.
func (c *consumer) setAside(msg *stomp.Message, id string, attempts int, cause error) error {

	conn := c.mc.getConn()
	if conn == nil {
		return fmt.Errorf("activemq: there is no connection to %s", c.mc.addr)
	}

	contentType := msg.ContentType
	if contentType == "" {
		contentType = contentTypeJSON
	}

	return conn.Send(c.deadLetterDestination(), contentType, msg.Body,
		stomp.SendOpt.Receipt,
		stomp.SendOpt.Header(headerOriginalDestination, c.destination),
		stomp.SendOpt.Header(headerOriginalMessageID, id),
		stomp.SendOpt.Header(headerDeliveryAttempts, strconv.Itoa(attempts)),
		stomp.SendOpt.Header(headerFailureReason, cause.Error()),
	)
}

// deadLetterDestination names the destination for the messages this consumer
// gave up on.
func (c *consumer) deadLetterDestination() string {

	return c.destination + deadLetterSuffix
}

// redeliver ends the subscription and takes a new one, which returns every
// message the old subscription held, the failed one included, to the queue.
// The wait between the two keeps a failure that repeats from running hot.
//
// It reports false when the consumer must stop.
func (c *consumer) redeliver(sub *stomp.Subscription) (*stomp.Subscription, uint64, bool) {

	if sub.Active() {
		if err := sub.Unsubscribe(); err != nil {
			log.GetLogger().Error(fmt.Sprintf(
				"activemq: %s consumer could not end its subscription: %v", c.name, err))
		}
	}

	timer := time.NewTimer(c.retryDelay)
	select {
	case <-c.mc.done:
		timer.Stop()
		log.GetLogger().Info(fmt.Sprintf("activemq: %s consumer stopped (shutdown)", c.name))
		return nil, 0, false
	case <-timer.C:
	}

	return c.subscribeAgain(false)
}

// subscribeAgain takes a new subscription on the current connection, and
// rebuilds the connection first when reconnect says that it is gone. It
// reports false when the consumer must stop.
func (c *consumer) subscribeAgain(reconnect bool) (*stomp.Subscription, uint64, bool) {

	logger := log.GetLogger()

	for {
		if c.mc.isShuttingDown() {
			logger.Info(fmt.Sprintf("activemq: %s consumer stopped (shutdown)", c.name))
			return nil, 0, false
		}

		if reconnect {
			if err := c.mc.reconnectWithBackoff(c.name+" consumer", 0); err != nil {
				logger.Info(fmt.Sprintf("activemq: %s consumer exiting: %v", c.name, err))
				return nil, 0, false
			}
		}

		sub, generation, err := c.mc.subscribeCurrent(c.destination)
		if err == nil {
			return sub, generation, true
		}

		logger.Error(fmt.Sprintf("activemq: %s consumer could not subscribe to %s: %v",
			c.name, c.destination, err))
		reconnect = true
	}
}

// messageID returns the identifier the broker gave the message, which is the
// same on every delivery of that message.
func messageID(msg *stomp.Message) string {

	if msg.Header == nil {
		return "unknown"
	}
	if id := msg.Header.Get("message-id"); id != "" {
		return id
	}
	return "unknown"
}
