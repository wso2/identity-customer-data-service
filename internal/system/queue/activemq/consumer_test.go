/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
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
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

// queuedMessage is one message the broker holds for the consumer.
type queuedMessage struct {
	id   string
	body string
}

// sentMessage is one message the consumer wrote to the broker.
type sentMessage struct {
	destination string
	body        string
	headers     map[string]string
}

// deliveringBroker is a STOMP broker that holds messages, gives them to a
// subscription, and records what the consumer answers. It keeps a message
// until the consumer acknowledges it, so a test can see exactly what survives
// a failure.
type deliveringBroker struct {
	listener net.Listener
	// receiptForSend says whether a SEND that asks for a receipt gets one. A
	// broker that does not answer stands for one that did not take the
	// message.
	receiptForSend bool

	mu         sync.Mutex
	pending    []queuedMessage
	inFlight   map[string]queuedMessage
	acked      []string
	sent       []sentMessage
	deliveries map[string]int
	conns      []net.Conn

	// The subscription that takes the next message, if there is one.
	activeConn        net.Conn
	activeSubscriber  string
	activeDestination string
}

// startDeliveringBroker listens on a local port and answers one client.
func startDeliveringBroker(t *testing.T, receiptForSend bool) *deliveringBroker {

	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	broker := &deliveringBroker{
		listener:       listener,
		receiptForSend: receiptForSend,
		inFlight:       map[string]queuedMessage{},
		deliveries:     map[string]int{},
	}

	t.Cleanup(func() {
		_ = listener.Close()
		broker.mu.Lock()
		for _, conn := range broker.conns {
			_ = conn.Close()
		}
		broker.mu.Unlock()
	})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			broker.mu.Lock()
			broker.conns = append(broker.conns, conn)
			broker.mu.Unlock()
			go broker.serve(conn)
		}
	}()

	return broker
}

func (b *deliveringBroker) addr() string {

	return "tcp://" + b.listener.Addr().String()
}

// enqueue adds a message and gives it to the subscription that is open, as a
// broker does for a consumer that is already reading.
func (b *deliveringBroker) enqueue(id, body string) {

	b.mu.Lock()
	b.pending = append(b.pending, queuedMessage{id: id, body: body})
	b.mu.Unlock()

	b.deliverPending()
}

// ackCount reports how many messages the consumer has acknowledged.
func (b *deliveringBroker) ackCount() int {

	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.acked)
}

// deliveryCount reports how often one message reached the consumer.
func (b *deliveringBroker) deliveryCount(id string) int {

	b.mu.Lock()
	defer b.mu.Unlock()
	return b.deliveries[id]
}

// sentTo returns the messages the consumer wrote to one destination.
func (b *deliveringBroker) sentTo(destination string) []sentMessage {

	b.mu.Lock()
	defer b.mu.Unlock()

	var found []sentMessage
	for _, message := range b.sent {
		if message.destination == destination {
			found = append(found, message)
		}
	}
	return found
}

// serve answers the frames of one client.
func (b *deliveringBroker) serve(conn net.Conn) {

	reader := bufio.NewReader(conn)
	for {
		frame, err := readFrame(reader)
		if err != nil {
			return
		}
		command, headers := parseFrame(frame)

		switch command {
		case "CONNECT", "STOMP":
			if _, err := conn.Write([]byte("CONNECTED\nversion:1.2\n\n\x00")); err != nil {
				return
			}
		case "SUBSCRIBE":
			b.subscribe(conn, headers["id"], headers["destination"])
		case "UNSUBSCRIBE":
			b.unsubscribe()
			b.answerReceipt(conn, headers["receipt"])
		case "ACK":
			b.recordAck(headers["id"])
		case "SEND":
			b.recordSend(headers, frameBody(frame))
			if b.receiptForSend {
				b.answerReceipt(conn, headers["receipt"])
			}
		case "DISCONNECT":
			b.answerReceipt(conn, headers["receipt"])
		}
	}
}

// subscribe records the subscription that takes the messages, and gives it
// what the broker already holds.
func (b *deliveringBroker) subscribe(conn net.Conn, subscriber, destination string) {

	b.mu.Lock()
	b.activeConn, b.activeSubscriber, b.activeDestination = conn, subscriber, destination
	b.mu.Unlock()

	b.deliverPending()
}

// unsubscribe ends the subscription and puts every message the consumer did
// not acknowledge back in the queue, which is what a broker does.
func (b *deliveringBroker) unsubscribe() {

	b.mu.Lock()
	defer b.mu.Unlock()

	b.activeConn, b.activeSubscriber, b.activeDestination = nil, "", ""
	for id, message := range b.inFlight {
		b.pending = append(b.pending, message)
		delete(b.inFlight, id)
	}
}

// deliverPending gives the open subscription every message the broker holds.
func (b *deliveringBroker) deliverPending() {

	b.mu.Lock()
	conn, subscriber, destination := b.activeConn, b.activeSubscriber, b.activeDestination
	if conn == nil {
		b.mu.Unlock()
		return
	}
	pending := b.pending
	b.pending = nil
	for _, message := range pending {
		b.inFlight[message.id] = message
		b.deliveries[message.id]++
	}
	b.mu.Unlock()

	for _, message := range pending {
		frame := fmt.Sprintf("MESSAGE\nsubscription:%s\nmessage-id:%s\nack:%s\ndestination:%s\n"+
			"content-type:application/json\n\n%s\x00",
			subscriber, message.id, message.id, destination, message.body)
		if _, err := conn.Write([]byte(frame)); err != nil {
			return
		}
	}
}

func (b *deliveringBroker) recordAck(id string) {

	b.mu.Lock()
	defer b.mu.Unlock()
	b.acked = append(b.acked, id)
	delete(b.inFlight, id)
}

func (b *deliveringBroker) recordSend(headers map[string]string, body string) {

	b.mu.Lock()
	defer b.mu.Unlock()
	b.sent = append(b.sent, sentMessage{
		destination: headers["destination"],
		body:        body,
		headers:     headers,
	})
}

func (b *deliveringBroker) answerReceipt(conn net.Conn, receipt string) {

	if receipt == "" {
		return
	}
	_, _ = fmt.Fprintf(conn, "RECEIPT\nreceipt-id:%s\n\n\x00", receipt)
}

// frameBody returns the body of one frame, which follows the blank line after
// the headers.
func frameBody(frame string) string {

	_, body, found := strings.Cut(strings.TrimLeft(frame, "\n"), "\n\n")
	if !found {
		return ""
	}
	return body
}

// newTestConsumer builds a consumer on a live connection to the broker, with a
// delivery policy the test can run through quickly.
func newTestConsumer(t *testing.T, broker *deliveringBroker, destination string, maxAttempts int,
	handle func([]byte) error) *managedConn {

	t.Helper()

	mc, err := newManagedConn(broker.addr(), "", "", config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect to the broker: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = mc.closeWithin(ctx)
	})

	c := newConsumer(mc, destination, "test", handle)
	c.maxAttempts = maxAttempts
	c.retryDelay = 20 * time.Millisecond

	if err := c.start(); err != nil {
		t.Fatalf("failed to start the consumer: %v", err)
	}
	return mc
}

// waitFor runs check until it is true or the time runs out.
func waitFor(t *testing.T, what string, check func() bool) {

	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Test_consumer_acknowledgesOnlyAfterTheJobIsDone checks the order that makes
// the queue survive a crash: the broker keeps the message for as long as the
// work runs.
func Test_consumer_acknowledgesOnlyAfterTheJobIsDone(t *testing.T) {

	broker := startDeliveringBroker(t, true)
	broker.enqueue("m1", `{"profile_id":"p1"}`)

	ackedWhileRunning := make(chan int, 1)
	done := make(chan struct{})

	newTestConsumer(t, broker, "/queue/test", 3, func([]byte) error {
		ackedWhileRunning <- broker.ackCount()
		close(done)
		return nil
	})

	<-done
	if n := <-ackedWhileRunning; n != 0 {
		t.Errorf("%d messages were acknowledged before the job finished, want 0", n)
	}

	waitFor(t, "the acknowledgement", func() bool { return broker.ackCount() >= 1 })
	time.Sleep(200 * time.Millisecond)

	if n := broker.ackCount(); n != 1 {
		t.Errorf("the message was acknowledged %d times, want 1", n)
	}
	if n := broker.deliveryCount("m1"); n != 1 {
		t.Errorf("the message was delivered %d times, want 1", n)
	}
	if sent := broker.sentTo("/queue/test" + deadLetterSuffix); len(sent) != 0 {
		t.Errorf("a message that succeeded was written to %s", "/queue/test"+deadLetterSuffix)
	}
}

// Test_consumer_takesAFailedMessageAgain checks that work which failed once is
// not lost and not acknowledged, and that it runs again.
func Test_consumer_takesAFailedMessageAgain(t *testing.T) {

	broker := startDeliveringBroker(t, true)
	broker.enqueue("m1", `{"profile_id":"p1"}`)

	var mu sync.Mutex
	attempts := 0

	newTestConsumer(t, broker, "/queue/test", 3, func([]byte) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		if attempts == 1 {
			return fmt.Errorf("the database is not available")
		}
		return nil
	})

	waitFor(t, "the second attempt to succeed", func() bool { return broker.ackCount() == 1 })

	if n := broker.deliveryCount("m1"); n != 2 {
		t.Errorf("the message was delivered %d times, want 2", n)
	}
	if sent := broker.sentTo("/queue/test" + deadLetterSuffix); len(sent) != 0 {
		t.Error("a message that succeeded on the second attempt was written to the dead letter destination")
	}
}

// Test_consumer_setsAsideAMessageThatNeverSucceeds checks that one message
// cannot hold the queue: it leaves after its attempts, and the message behind
// it is processed.
func Test_consumer_setsAsideAMessageThatNeverSucceeds(t *testing.T) {

	broker := startDeliveringBroker(t, true)
	broker.enqueue("poison", `{"profile_id":"bad"}`)

	var mu sync.Mutex
	var handled []string

	newTestConsumer(t, broker, "/queue/test", 3, func(body []byte) error {
		mu.Lock()
		handled = append(handled, string(body))
		mu.Unlock()
		if strings.Contains(string(body), "bad") {
			return fmt.Errorf("this job always fails")
		}
		return nil
	})

	deadLetter := "/queue/test" + deadLetterSuffix
	waitFor(t, "the message to be set aside", func() bool { return len(broker.sentTo(deadLetter)) == 1 })

	if n := broker.deliveryCount("poison"); n != 3 {
		t.Errorf("the message was delivered %d times, want 3", n)
	}

	message := broker.sentTo(deadLetter)[0]
	if message.body != `{"profile_id":"bad"}` {
		t.Errorf("the dead letter carries %q, want the original body", message.body)
	}
	if got := message.headers[headerOriginalMessageID]; got != "poison" {
		t.Errorf("the dead letter names message %q, want poison", got)
	}
	if got := message.headers[headerOriginalDestination]; got != "/queue/test" {
		t.Errorf("the dead letter names destination %q, want /queue/test", got)
	}
	if got := message.headers[headerDeliveryAttempts]; got != "3" {
		t.Errorf("the dead letter reports %q attempts, want 3", got)
	}

	// The message leaves the queue only after the copy is safe.
	waitFor(t, "the acknowledgement", func() bool { return broker.ackCount() == 1 })

	// The queue still moves.
	broker.enqueue("good", `{"profile_id":"ok"}`)
	waitFor(t, "the message behind it", func() bool { return broker.deliveryCount("good") == 1 })
	waitFor(t, "the second acknowledgement", func() bool { return broker.ackCount() == 2 })
}

// Test_consumer_setsAsideAMessageItCannotRead checks that a body which will
// never parse is not tried again.
func Test_consumer_setsAsideAMessageItCannotRead(t *testing.T) {

	broker := startDeliveringBroker(t, true)
	broker.enqueue("m1", "this is not JSON")

	mc, err := newManagedConn(broker.addr(), "", "", config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect to the broker: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mc.closeWithin(ctx)
	})

	q := &ProfileQueue{mc: mc, destination: "/queue/test"}
	handled := make(chan profileModel.Profile, 1)
	if err := q.Start(func(profile profileModel.Profile) error {
		handled <- profile
		return nil
	}); err != nil {
		t.Fatalf("failed to start the queue: %v", err)
	}

	deadLetter := "/queue/test" + deadLetterSuffix
	waitFor(t, "the message to be set aside", func() bool { return len(broker.sentTo(deadLetter)) == 1 })

	if n := broker.deliveryCount("m1"); n != 1 {
		t.Errorf("the message was delivered %d times, want 1", n)
	}
	select {
	case profile := <-handled:
		t.Errorf("a message that cannot be read reached the handler as %v", profile)
	default:
	}
	waitFor(t, "the acknowledgement", func() bool { return broker.ackCount() == 1 })
}

// Test_consumer_keepsTheMessageWhenTheDeadLetterIsNotTaken checks the order of
// the two steps. A broker that does not confirm the copy must not lose the
// original.
func Test_consumer_keepsTheMessageWhenTheDeadLetterIsNotTaken(t *testing.T) {

	broker := startDeliveringBroker(t, false)
	broker.enqueue("poison", `{"profile_id":"bad"}`)

	newTestConsumer(t, broker, "/queue/test", 1, func([]byte) error {
		return fmt.Errorf("this job always fails")
	})

	// The send waits for a receipt that never comes, and the message is then
	// delivered again rather than acknowledged.
	waitFor(t, "a second delivery", func() bool { return broker.deliveryCount("poison") >= 2 })

	if n := broker.ackCount(); n != 0 {
		t.Errorf("%d messages were acknowledged although the dead letter was not confirmed, want 0", n)
	}
}

// Test_consumer_leavesTheMessageWhenTheQueueIsClosing checks that a job
// refused during shutdown neither acknowledges the message nor takes a new
// subscription. The close returns the work to the queue.
func Test_consumer_leavesTheMessageWhenTheQueueIsClosing(t *testing.T) {

	broker := startDeliveringBroker(t, true)
	broker.enqueue("m1", `{"profile_id":"p1"}`)

	mc, err := newManagedConn(broker.addr(), "", "", config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect to the broker: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mc.closeWithin(ctx)
	})

	refused := make(chan struct{})

	// The queue starts to close while the message is at the handler.
	c := newConsumer(mc, "/queue/test", "test", func([]byte) error {
		mc.shutdown()
		defer close(refused)
		return fmt.Errorf("the worker is stopping")
	})
	c.retryDelay = 20 * time.Millisecond
	if err := c.start(); err != nil {
		t.Fatalf("failed to start the consumer: %v", err)
	}

	<-refused

	time.Sleep(200 * time.Millisecond)

	if n := broker.ackCount(); n != 0 {
		t.Errorf("%d messages were acknowledged during the close, want 0", n)
	}
	if n := broker.deliveryCount("m1"); n != 1 {
		t.Errorf("the message was delivered %d times during the close, want 1", n)
	}
}
