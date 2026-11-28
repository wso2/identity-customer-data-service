/*
 * Copyright (c) 2026, Choreo Labs LLC. (https://choreolabs.com).
 *
 * Choreo Labs licenses this file to you under the Apache License,
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
	"net"
	"sync"
	"testing"
	"time"

	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

// Test_ProfileQueue_producerReconnectKeepsTheSubscription checks that the producer and the consumer
// use separate connections. A reconnect after a failed send must not end the subscription: on a
// shared connection, the jobs in the hands of the workers could not be acknowledged, and the broker
// would deliver them again.
func Test_ProfileQueue_producerReconnectKeepsTheSubscription(t *testing.T) {
	broker := startFakeBroker(t, true)
	q, err := NewProfileQueue(broker.addr(), "", "", "/queue/profiles", config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect to the fake broker: %v", err)
	}
	defer closeQueue(t, q.Close)

	handled := make(chan string, 1)
	if err := q.Start(func(p profileModel.Profile) error {
		handled <- p.ProfileID
		return nil
	}, 1); err != nil {
		t.Fatalf("failed to start the queue: %v", err)
	}
	nextFrame(t, broker.subscribes, "SUBSCRIBE")
	if got := broker.connections(); got != 2 {
		t.Fatalf("the queue opened %d connections, want 2 (one to send, one to consume)", got)
	}

	if err := q.producer.dial(); err != nil {
		t.Fatalf("the producer did not reconnect: %v", err)
	}
	noFrame(t, broker.subscribes, "SUBSCRIBE")

	broker.deliver(t, "msg-1", `{"profile_id":"p-1"}`)
	select {
	case id := <-handled:
		if id != "p-1" {
			t.Fatalf("the handler got profile %q, want p-1", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the subscription did not deliver the job after the producer reconnected")
	}
	if got := nextFrame(t, broker.acks, "ACK")["id"]; got != "msg-1" {
		t.Errorf("ACK id = %q, want msg-1", got)
	}
}

// Test_SchemaSyncQueue_usesSeparateConnections checks the same split for the schema sync queue.
func Test_SchemaSyncQueue_usesSeparateConnections(t *testing.T) {
	broker := startFakeBroker(t, true)
	q, err := NewSchemaSyncQueue(broker.addr(), "", "", "/queue/schema", config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect to the fake broker: %v", err)
	}
	defer closeQueue(t, q.Close)

	if err := q.Start(func(schemaModel.ProfileSchemaSync) {}); err != nil {
		t.Fatalf("failed to start the queue: %v", err)
	}
	nextFrame(t, broker.subscribes, "SUBSCRIBE")
	if got := broker.connections(); got != 2 {
		t.Fatalf("the queue opened %d connections, want 2 (one to send, one to consume)", got)
	}

	if err := q.producer.dial(); err != nil {
		t.Fatalf("the producer did not reconnect: %v", err)
	}
	noFrame(t, broker.subscribes, "SUBSCRIBE")
}

// Test_dial_doesNotWaitForTheReplacedConnection checks that a reconnect does not wait for the old
// connection. go-stomp waits up to 30 seconds for a disconnect receipt, and a reconnect holds the
// lock that the sends of the request path wait on. The old socket closes by force after
// retireTimeout.
func Test_dial_doesNotWaitForTheReplacedConnection(t *testing.T) {
	retireAfter := retireTimeout
	retireTimeout = 200 * time.Millisecond
	t.Cleanup(func() { retireTimeout = retireAfter })

	broker := startFakeBroker(t, false)
	q, err := NewProfileQueue(broker.addr(), "", "", "/queue/profiles", config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect to the fake broker: %v", err)
	}
	// The broker never answers a disconnect, so Close ends at its deadline.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = q.Close(ctx)
	}()

	start := time.Now()
	if err := q.producer.dial(); err != nil {
		t.Fatalf("the producer did not reconnect: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the reconnect took %s, so it waited for the old connection", elapsed)
	}

	select {
	case <-broker.readEnded:
	case <-time.After(5 * time.Second):
		t.Fatal("the old connection was not closed after retireTimeout")
	}
}

// startHandshakeBroker listens on a local port and answers the CONNECT of the
// first answered connections only. Later connections are accepted and read,
// but get no CONNECTED frame, like a broker that hangs.
func startHandshakeBroker(t *testing.T, answered int) (addr string, accepted func() int) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	var mu sync.Mutex
	var conns []net.Conn
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		mu.Unlock()
	})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			answer := len(conns) <= answered
			mu.Unlock()
			go func() {
				reader := bufio.NewReader(conn)
				for {
					frame, err := readFrame(reader)
					if err != nil {
						return
					}
					command, headers := parseFrame(frame)
					switch {
					case answer && (command == frameConnect || command == frameStomp):
						_, _ = conn.Write([]byte("CONNECTED\nversion:1.2\n\n\x00"))
					case answer && command == frameDisconnect:
						_, _ = conn.Write([]byte("RECEIPT\nreceipt-id:" + headers["receipt"] + "\n\n\x00"))
					}
				}
			}()
		}
	}()

	return "tcp://" + listener.Addr().String(), func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(conns)
	}
}

// Test_dial_endsAHandshakeThatTheBrokerDoesNotAnswer checks that a broker that
// accepts the socket but never answers CONNECT does not hold the dial for good.
func Test_dial_endsAHandshakeThatTheBrokerDoesNotAnswer(t *testing.T) {
	connectAfter := connectTimeout
	connectTimeout = 200 * time.Millisecond
	t.Cleanup(func() { connectTimeout = connectAfter })

	addr, _ := startHandshakeBroker(t, 0)
	start := time.Now()
	if _, err := NewProfileQueue(addr, "", "", "/queue/profiles", config.TLSConfig{}); err == nil {
		t.Fatal("the queue connected to a broker that did not answer CONNECT")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the dial took %s, want about connectTimeout", elapsed)
	}
}

// Test_Close_doesNotWaitForAConsumerThatConnects checks that Close returns
// inside its context while Start dials the consumer connection, and that the
// late Start then fails without leaving a consumer behind.
func Test_Close_doesNotWaitForAConsumerThatConnects(t *testing.T) {
	connectAfter := connectTimeout
	connectTimeout = 2 * time.Second
	t.Cleanup(func() { connectTimeout = connectAfter })

	addr, accepted := startHandshakeBroker(t, 1)
	q, err := NewProfileQueue(addr, "", "", "/queue/profiles", config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect the producer: %v", err)
	}

	started := make(chan error, 1)
	go func() { started <- q.Start(func(profileModel.Profile) error { return nil }, 1) }()
	deadline := time.Now().Add(5 * time.Second)
	for accepted() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("Start did not dial the consumer connection")
		}
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_ = q.Close(ctx)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Close took %s, so it waited for the consumer connection", elapsed)
	}

	select {
	case err := <-started:
		if err == nil {
			t.Fatal("Start succeeded after Close")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.consumer != nil {
		t.Error("the queue kept a consumer connection after Close")
	}
}
