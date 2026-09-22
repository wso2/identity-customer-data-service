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

package queueintegration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-stomp/stomp/v3"
	"github.com/google/uuid"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
	"github.com/wso2/identity-customer-data-service/internal/system/queue/activemq"
	"github.com/wso2/identity-customer-data-service/test/setup"
)

var brokerAddr, brokerUser, brokerPassword string

func TestMain(m *testing.M) {
	_ = log.Init("ERROR")
	config.OverrideCDSRuntime(config.Config{})
	brokerAddr = os.Getenv("CDS_TEST_ACTIVEMQ_ADDR")
	brokerUser, brokerPassword = "admin", "admin"
	var cleanup func()
	if brokerAddr == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		broker, err := setup.SetupTestActiveMQ(ctx)
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		brokerAddr, brokerUser, brokerPassword = broker.Addr, broker.Username, broker.Password
		cleanup = func() { _ = broker.Container.Terminate(context.Background()) }
	}
	code := m.Run()
	if cleanup != nil {
		cleanup()
	}
	os.Exit(code)
}

func connection(t *testing.T) *stomp.Conn {
	t.Helper()
	conn, err := stomp.Dial("tcp", brokerAddr, stomp.ConnOpt.Login(brokerUser, brokerPassword),
		stomp.ConnOpt.AcceptVersion(stomp.V12), stomp.ConnOpt.HeartBeat(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.MustDisconnect() })
	return conn
}

func subscribe(t *testing.T, conn *stomp.Conn, destination string) *stomp.Subscription {
	t.Helper()
	sub, err := conn.Subscribe(destination, stomp.AckClientIndividual)
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

func receive(t *testing.T, sub *stomp.Subscription) *stomp.Message {
	t.Helper()
	select {
	case msg := <-sub.C:
		if msg == nil || msg.Err != nil {
			t.Fatalf("receive failed: %v", msg)
		}
		return msg
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for broker message")
		return nil
	}
}

func closeQueue(t *testing.T, q interface{ Close(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := q.Close(ctx); err != nil {
		t.Error(err)
	}
}

func TestProcessingOutcomes(t *testing.T) {
	for _, test := range []struct {
		name      string
		failUntil int32
		retryable bool
		wantCalls int32
		wantDLQ   bool
	}{
		{"success", 0, false, 1, false},
		{"transient", 1, true, 2, false},
		{"exhausted", 100, true, 3, true},
		{"terminal", 100, false, 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			destination := "/queue/cds-pr4-" + uuid.NewString()
			conn := connection(t)
			dlq := subscribe(t, conn, destination+".DLQ")
			q, err := activemq.NewProfileQueue(brokerAddr, brokerUser, brokerPassword, destination, config.TLSConfig{})
			if err != nil {
				t.Fatal(err)
			}
			defer closeQueue(t, q)
			var calls atomic.Int32
			completed := make(chan struct{}, 1)
			if err := q.Start(func(profileModel.Profile) error {
				if calls.Add(1) <= test.failUntil {
					err := errors.New("test failure containing sensitive data")
					if test.name == "transient" {
						err = context.DeadlineExceeded
					}
					if test.retryable {
						return queue.Retryable(err)
					}
					return err
				}
				completed <- struct{}{}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := q.Enqueue(profileModel.Profile{ProfileId: "test-profile"}); err != nil {
				t.Fatal(err)
			}
			if test.wantDLQ {
				msg := receive(t, dlq)
				if msg.Header.Get("cds-original-destination") != destination {
					t.Fatal("missing source metadata")
				}
				if msg.Header.Get("cds-original-message-id") == "" {
					t.Fatal("missing correlation ID")
				}
				if msg.Header.Get("cds-failure-category") == "" {
					t.Fatal("missing failure category")
				}
				if msg.Header.Get("persistent") != "true" {
					t.Fatal("DLQ copy is not persistent")
				}
				if err := conn.Ack(msg); err != nil {
					t.Fatal(err)
				}
			} else {
				select {
				case <-completed:
				case <-time.After(15 * time.Second):
					t.Fatal("handler did not succeed")
				}
			}
			if got := calls.Load(); got != test.wantCalls {
				t.Fatalf("handler called %d times, want %d", got, test.wantCalls)
			}
			if !test.wantDLQ {
				// Handler completion precedes final settlement. Closing at that
				// point may legitimately redeliver; ACK ordering is tested at
				// protocol level instead of relying on goroutine scheduling here.
				select {
				case msg := <-dlq.C:
					t.Fatalf("successful work dead-lettered: %v", msg)
				case <-time.After(200 * time.Millisecond):
				}
				return
			}
			closeQueue(t, q)
			// If the source ACK and replacement SEND were not atomic, a source
			// message could still be delivered after a confirmed DLQ transfer.
			source := subscribe(t, conn, destination)
			select {
			case msg := <-source.C:
				t.Fatalf("source was not settled: %v", msg)
			case <-time.After(200 * time.Millisecond):
			}
		})
	}
}

func TestMalformedMessageGoesToDLQ(t *testing.T) {
	destination := "/queue/cds-pr4-" + uuid.NewString()
	conn := connection(t)
	dlq := subscribe(t, conn, destination+".DLQ")
	q, err := activemq.NewSchemaSyncQueue(brokerAddr, brokerUser, brokerPassword, destination, config.TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeQueue(t, q)
	var calls atomic.Int32
	if err := q.Start(func(schemaModel.ProfileSchemaSync) error { calls.Add(1); return nil }); err != nil {
		t.Fatal(err)
	}
	body := []byte("not-json")
	if err := conn.Send(destination, "application/json", body, stomp.SendOpt.Receipt); err != nil {
		t.Fatal(err)
	}
	msg := receive(t, dlq)
	if string(msg.Body) != string(body) {
		t.Fatal("DLQ changed original payload")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid payload reached handler")
	}
}

func TestDeferredMessageIsRedelivered(t *testing.T) {
	destination := "/queue/cds-pr4-" + uuid.NewString()
	q, err := activemq.NewProfileQueue(brokerAddr, brokerUser, brokerPassword, destination, config.TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeQueue(t, q)
	reached := make(chan struct{})
	if err := q.Start(func(profileModel.Profile) error { close(reached); return queue.ErrDeferred }); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(profileModel.Profile{ProfileId: "deferred"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not run")
	}
	closeQueue(t, q)
	conn := connection(t)
	msg := receive(t, subscribe(t, conn, destination))
	if msg.Header.Get("persistent") != "true" {
		t.Fatal("source message is not persistent")
	}
	if msg.Header.Get("cds-processing-attempt") != "" {
		t.Fatal("shutdown consumed a processing attempt")
	}
	if err := conn.Ack(msg); err != nil {
		t.Fatal(err)
	}
}

// A TCP proxy lets the test interrupt a real broker connection without
// shutting down a broker that may be shared by other test packages.
func interruptibleProxy(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var sockets []net.Conn
	closed := false
	interrupt := func() {
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range sockets {
			_ = conn.Close()
		}
		sockets = nil
	}
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		closed = true
		mu.Unlock()
		interrupt()
	})
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			broker, err := net.DialTimeout("tcp", brokerAddr, time.Second)
			if err != nil {
				_ = client.Close()
				continue
			}
			mu.Lock()
			if closed {
				mu.Unlock()
				_ = client.Close()
				_ = broker.Close()
				return
			}
			sockets = append(sockets, client, broker)
			mu.Unlock()
			go func() { _, _ = io.Copy(broker, client); _ = broker.Close() }()
			go func() { _, _ = io.Copy(client, broker); _ = client.Close() }()
		}
	}()
	return listener.Addr().String(), interrupt
}

func TestConsumerRecoversAfterConnectionLoss(t *testing.T) {
	addr, interrupt := interruptibleProxy(t)
	destination := "/queue/cds-pr4-" + uuid.NewString()
	q, err := activemq.NewProfileQueue(addr, brokerUser, brokerPassword, destination, config.TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeQueue(t, q)
	entered, release, completed := make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
	var once sync.Once
	defer once.Do(func() { close(release) })
	var calls atomic.Int32
	if err := q.Start(func(profileModel.Profile) error {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		} else {
			completed <- struct{}{}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(profileModel.Profile{ProfileId: "reconnect"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not start")
	}
	interrupt()
	once.Do(func() { close(release) })
	select {
	case <-completed:
	case <-time.After(15 * time.Second):
		t.Fatal("unacknowledged work was not redelivered after reconnect")
	}
	if calls.Load() != 2 {
		t.Fatal("unexpected delivery count")
	}
}

func TestInFlightMessageRemainsUnacknowledged(t *testing.T) {
	destination := "/queue/cds-pr4-" + uuid.NewString()
	q, err := activemq.NewProfileQueue(brokerAddr, brokerUser, brokerPassword, destination, config.TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeQueue(t, q)
	reached, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer func() {
		close(release)
		select {
		case <-returned:
		case <-time.After(time.Second):
			t.Error("handler did not exit after release")
		}
	}()
	if err := q.Start(func(profileModel.Profile) error {
		close(reached)
		<-release
		close(returned)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(profileModel.Profile{ProfileId: "active"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not run")
	}
	// Simulate the shutdown deadline ending before the handler finishes.
	closeQueue(t, q)
	conn := connection(t)
	msg := receive(t, subscribe(t, conn, destination))
	if err := conn.Ack(msg); err != nil {
		t.Fatal(err)
	}
}
