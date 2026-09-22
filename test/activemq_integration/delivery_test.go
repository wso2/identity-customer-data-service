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

package activemqintegration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-stomp/stomp/v3"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/queue/activemq"
)

// brokerConfig returns the broker the test containers started.
func brokerConfig(t *testing.T) config.ExternalBrokerConfig {

	t.Helper()
	return config.GetCDSRuntime().Config.MessageQueue.Broker
}

// startTestQueue opens a queue of its own on the broker and reads it with
// handler. The queue closes when the test ends.
func startTestQueue(t *testing.T, destination string,
	handler func(profileModel.Profile) error) *activemq.ProfileQueue {

	t.Helper()

	broker := brokerConfig(t)
	q, err := activemq.NewProfileQueue(broker.Addr, broker.Username, broker.Password, destination,
		config.TLSConfig{})
	if err != nil {
		t.Fatalf("failed to connect to the broker: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = q.Close(ctx)
	})

	if err := q.Start(handler); err != nil {
		t.Fatalf("failed to start the queue: %v", err)
	}
	return q
}

// Test_ActiveMQ_KeepsWorkThatFailed checks against a real broker that a job
// which failed is neither lost nor acknowledged, and that it runs again.
func Test_ActiveMQ_KeepsWorkThatFailed(t *testing.T) {

	destination := "/queue/cds-test-delivery-retry"

	var mu sync.Mutex
	var attempts int
	var once sync.Once
	succeeded := make(chan struct{})

	q := startTestQueue(t, destination, func(profile profileModel.Profile) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		if attempts == 1 {
			return fmt.Errorf("the database is not available")
		}
		once.Do(func() { close(succeeded) })
		return nil
	})

	if err := q.Enqueue(profileModel.Profile{ProfileId: "delivery-retry-1"}); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	select {
	case <-succeeded:
	case <-time.After(60 * time.Second):
		t.Fatal("the failed job was not delivered again")
	}

	// A message the handler completed does not come back.
	time.Sleep(5 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if attempts != 2 {
		t.Errorf("the job ran %d times, want 2", attempts)
	}
}

// Test_ActiveMQ_SetsAsideWorkThatNeverSucceeds checks where a message that
// always fails ends up. The broker of this chart has no redelivery policy of
// its own, so the consumer writes the message to the dead letter destination
// itself, and this test reads it from there.
func Test_ActiveMQ_SetsAsideWorkThatNeverSucceeds(t *testing.T) {

	destination := "/queue/cds-test-delivery-poison"
	deadLetter := destination + ".DLQ"

	broker := brokerConfig(t)
	reader, err := stomp.Dial("tcp", broker.Addr, stomp.ConnOpt.Login(broker.Username, broker.Password))
	if err != nil {
		t.Fatalf("failed to connect to the broker: %v", err)
	}
	defer func() { _ = reader.Disconnect() }()

	deadLetterSub, err := reader.Subscribe(deadLetter, stomp.AckClientIndividual)
	if err != nil {
		t.Fatalf("failed to subscribe to %s: %v", deadLetter, err)
	}

	var mu sync.Mutex
	var attempts int
	var once sync.Once
	good := make(chan struct{})

	q := startTestQueue(t, destination, func(profile profileModel.Profile) error {
		if profile.ProfileId == "poison" {
			mu.Lock()
			attempts++
			mu.Unlock()
			return fmt.Errorf("this job always fails")
		}
		once.Do(func() { close(good) })
		return nil
	})

	if err := q.Enqueue(profileModel.Profile{ProfileId: "poison"}); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	select {
	case msg := <-deadLetterSub.C:
		if msg.Err != nil {
			t.Fatalf("the dead letter subscription failed: %v", msg.Err)
		}
		if origin := msg.Header.Get("cds-original-destination"); origin != destination {
			t.Errorf("the dead letter names destination %q, want %q", origin, destination)
		}
		if attemptsHeader := msg.Header.Get("cds-delivery-attempts"); attemptsHeader == "" {
			t.Error("the dead letter does not say how often the job was tried")
		}
		if reason := msg.Header.Get("cds-failure-reason"); reason == "" {
			t.Error("the dead letter does not say why the job failed")
		}
		_ = reader.Ack(msg)
	case <-time.After(120 * time.Second):
		t.Fatal("the message that always fails did not reach the dead letter destination")
	}

	mu.Lock()
	tried := attempts
	mu.Unlock()
	if tried < 2 {
		t.Errorf("the job ran %d times before it was set aside, want more than one attempt", tried)
	}

	// The queue behind the message that failed still moves.
	if err := q.Enqueue(profileModel.Profile{ProfileId: "good"}); err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}
	select {
	case <-good:
	case <-time.After(60 * time.Second):
		t.Fatal("the message behind the one that failed was not processed")
	}
}
