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
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/go-stomp/stomp/v3/frame"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

// protocolBroker observes real STOMP frames and can withhold a COMMIT receipt.
func protocolBroker(t *testing.T, commitReceipt bool) (string, <-chan *frame.Frame) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	frames := make(chan *frame.Frame, 32)
	var mu sync.Mutex
	var socket net.Conn
	t.Cleanup(func() {
		_ = listener.Close()
		mu.Lock()
		defer mu.Unlock()
		if socket != nil {
			_ = socket.Close()
		}
	})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		mu.Lock()
		socket = conn
		mu.Unlock()
		defer conn.Close()
		reader, writer := frame.NewReader(conn), frame.NewWriter(conn)
		for {
			f, err := reader.Read()
			if err != nil {
				return
			}
			if f == nil {
				continue
			}
			frames <- f
			switch f.Command {
			case frame.CONNECT, frame.STOMP:
				if err := writer.Write(frame.New(frame.CONNECTED, "version", "1.2")); err != nil {
					return
				}
			case frame.SUBSCRIBE:
				msg := frame.New(frame.MESSAGE, "subscription", f.Header.Get("id"), "message-id", "original",
					"ack", "ack-original", "destination", f.Header.Get("destination"), "content-type", "application/json")
				msg.Body = []byte(`{"profile_id":"test"}`)
				if err := writer.Write(msg); err != nil {
					return
				}
			case frame.COMMIT, frame.DISCONNECT:
				if f.Command == frame.COMMIT && !commitReceipt {
					continue
				}
				if receipt := f.Header.Get("receipt"); receipt != "" {
					if err := writer.Write(frame.New(frame.RECEIPT, "receipt-id", receipt)); err != nil {
						return
					}
				}
			}
		}
	}()
	return listener.Addr().String(), frames
}

func awaitFrame(t *testing.T, frames <-chan *frame.Frame, command string, timeout time.Duration) *frame.Frame {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case f := <-frames:
			if f.Command == command {
				return f
			}
		case <-timer.C:
			t.Fatalf("did not receive %s", command)
			return nil
		}
	}
}

func TestAcknowledgementWaitsForHandler(t *testing.T) {
	addr, frames := protocolBroker(t, true)
	q, err := NewProfileQueue(addr, "", "", "/queue/test", config.TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = q.Close(ctx)
	})
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	if err := q.Start(func(profileModel.Profile) error { close(entered); <-release; return nil }); err != nil {
		t.Fatal(err)
	}
	sub := awaitFrame(t, frames, frame.SUBSCRIBE, time.Second)
	if sub.Header.Get("ack") != "client-individual" || sub.Header.Get("activemq.prefetchSize") != "1" {
		t.Fatal("unsafe subscription options")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler not entered")
	}
	select {
	case f := <-frames:
		t.Fatalf("frame %s sent before handler returned", f.Command)
	case <-time.After(100 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	ack := awaitFrame(t, frames, frame.ACK, time.Second)
	if ack.Header.Get("id") != "ack-original" {
		t.Fatal("wrong message acknowledged")
	}
}

func TestDeadLetterCommitReceiptIsBounded(t *testing.T) {
	addr, frames := protocolBroker(t, false)
	q, err := NewProfileQueue(addr, "", "", "/queue/test", config.TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = q.Close(ctx)
	})
	if err := q.Start(func(profileModel.Profile) error { return errors.New("terminal") }); err != nil {
		t.Fatal(err)
	}
	send := awaitFrame(t, frames, frame.SEND, time.Second)
	if send.Header.Get("destination") != "/queue/test.DLQ" || send.Header.Get("persistent") != "true" {
		t.Fatal("invalid dead-letter send")
	}
	ack := awaitFrame(t, frames, frame.ACK, time.Second)
	if ack.Header.Get("transaction") == "" || ack.Header.Get("transaction") != send.Header.Get("transaction") {
		t.Fatal("source acknowledgement not atomic with DLQ send")
	}
	awaitFrame(t, frames, frame.COMMIT, time.Second)
	// A silent broker used to leave go-stomp waiting forever when heartbeats
	// were disabled. The bounded transfer closes the socket instead.
	q.mc.mu.RLock()
	socket := q.mc.netConn
	q.mc.mu.RUnlock()
	deadline := time.After(brokerIOTimeout + 2*time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := socket.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
				return
			}
		case <-deadline:
			t.Fatal("silent commit receipt left connection open")
		}
	}
}

func TestShutdownInterruptsPendingHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	mc := &managedConn{addr: listener.Addr().String(), done: make(chan struct{}), reconnectGate: make(chan struct{}, 1)}
	result := make(chan error, 1)
	go func() { result <- mc.dial() }()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := frame.NewReader(conn).Read(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := mc.closeWithin(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("handshake completed after shutdown")
		}
	case <-ctx.Done():
		t.Fatal("shutdown did not interrupt handshake")
	}
}
