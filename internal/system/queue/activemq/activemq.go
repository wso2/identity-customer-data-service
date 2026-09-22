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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-stomp/stomp/v3"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

const (
	contentTypeJSON   = "application/json"
	receiptTimeout    = 5 * time.Second
	initialBackoff    = 2 * time.Second
	maxBackoff        = 60 * time.Second
	backoffMultiplier = 2
)

func init() {
	queue.RegisterProfileQueueProvider(queue.TypeActiveMQ,
		func(cfg config.ExternalBrokerConfig, tlsCfg config.TLSConfig) (queue.ProfileUnificationQueue, error) {
			return NewProfileQueue(cfg.Addr, cfg.Username, cfg.Password, cfg.ProfileQueueName, tlsCfg)
		},
	)
	queue.RegisterSchemaSyncQueueProvider(queue.TypeActiveMQ,
		func(cfg config.ExternalBrokerConfig, tlsCfg config.TLSConfig) (queue.SchemaSyncQueue, error) {
			return NewSchemaSyncQueue(cfg.Addr, cfg.Username, cfg.Password, cfg.SchemaSyncQueueName, tlsCfg)
		},
	)
}

// managedConn holds a STOMP connection and re-dials transparently when the
// connection is lost. The conn field is protected by mu to prevent data races
// between concurrent Enqueue calls, consumer goroutines, and reconnect
// attempts. The generation is incremented every time a new connection is
// installed so consumers can detect whether a closed subscription belongs to
// an intentionally retired connection or to the current live connection.
// The done channel is used to signal consumer goroutines to exit during
// graceful shutdown. The netConn field holds the socket of the current
// connection, so that a close which does not finish inside its deadline can be
// ended by force.
type managedConn struct {
	addr     string
	username string
	password string
	tlsCfg   config.TLSConfig

	mu          sync.RWMutex
	conn        *stomp.Conn
	netConn     net.Conn
	generation  uint64
	reconnectMu sync.Mutex
	done        chan struct{}
	doneOnce    sync.Once
	// closed guards the install in dial against a concurrent shutdown. It is
	// read and written under mu, which is also the lock that installs a
	// connection, so a connection is never installed after shutdown.
	closed bool

	// closeOnce disconnects once and keeps the result, so a repeated Close
	// returns the same answer.
	closeOnce sync.Once
	closeErr  error
}

func newManagedConn(addr, username, password string, tlsCfg config.TLSConfig) (*managedConn, error) {
	mc := &managedConn{
		addr:     addr,
		username: username,
		password: password,
		tlsCfg:   tlsCfg,
		done:     make(chan struct{}),
	}
	if err := mc.dial(); err != nil {
		return nil, err
	}
	return mc, nil
}

// parseAddr strips a transport scheme from addr and returns the bare
// "host:port" and whether TLS should be used.
//
// Supported schemes:
//   - "ssl://"  → TLS (ActiveMQ SSL transport)
//   - "tcp://"  → plain TCP
//   - no scheme → plain TCP (bare "host:port")
func parseAddr(addr string) (hostPort string, useTLS bool) {
	switch {
	case strings.HasPrefix(addr, "ssl://"):
		return strings.TrimPrefix(addr, "ssl://"), true
	case strings.HasPrefix(addr, "tcp://"):
		return strings.TrimPrefix(addr, "tcp://"), false
	}
	return addr, false
}

func (mc *managedConn) dial() error {
	hostPort, useTLS := parseAddr(mc.addr)

	opts := []func(*stomp.Conn) error{
		stomp.ConnOpt.Login(mc.username, mc.password),
		// Disable STOMP-level heartbeats to avoid spurious read-timeout
		// disconnects when the broker sends heartbeats less frequently than
		// the negotiated interval. TCP keepalive (set below) provides
		// equivalent liveness detection for half-open connections.
		stomp.ConnOpt.HeartBeat(0, 0),
		// go-stomp allows 30 seconds for each of these receipts, which is
		// longer than the whole shutdown of the application. A broker that
		// stops answering must not hold a consumer, and must not hold the
		// close of the connection past its deadline either.
		stomp.ConnOpt.UnsubscribeReceiptTimeout(receiptTimeout),
		stomp.ConnOpt.RcvReceiptTimeout(receiptTimeout),
		stomp.ConnOpt.DisconnectReceiptTimeout(receiptTimeout),
		stomp.ConnOpt.MsgSendTimeout(receiptTimeout),
	}

	// Use a dialer with an explicit connect timeout and TCP keepalive.
	// The timeout prevents indefinite blocking during reconnects. Keepalive
	// lets the OS probe idle connections and surface half-open or
	// load-balancer-dropped sockets without relying on STOMP heartbeats.
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	var netConn net.Conn
	var err error
	if useTLS {
		// Build CA pool from system roots, then append the trust store if
		// configured — same pattern as identity_client.go. This allows the
		// broker's internal CA cert to be added to the existing trust_store
		// without any new config fields.
		rootCAs, sysErr := x509.SystemCertPool()
		if sysErr != nil || rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		if mc.tlsCfg.TrustStore != "" {
			certDir := mc.tlsCfg.CertDir
			if certDir == "" {
				certDir = filepath.Join(utils.GetCDSHome(), "etc", "certs")
			}
			if !filepath.IsAbs(certDir) {
				if abs, absErr := filepath.Abs(certDir); absErr == nil {
					certDir = abs
				}
			}
			trustPEM, readErr := os.ReadFile(filepath.Join(certDir, mc.tlsCfg.TrustStore))
			if readErr != nil {
				return fmt.Errorf("activemq: failed to read trust_store: %w", readErr)
			}
			if ok := rootCAs.AppendCertsFromPEM(trustPEM); !ok {
				return fmt.Errorf("activemq: failed to append certs from trust_store")
			}
		}
		netConn, err = tls.DialWithDialer(dialer, "tcp", hostPort, &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    rootCAs,
		})
	} else {
		netConn, err = dialer.Dial("tcp", hostPort)
	}
	if err != nil {
		return fmt.Errorf("activemq: dial %s: %w", mc.addr, err)
	}

	stompConn, err := stomp.Connect(netConn, opts...)
	if err != nil {
		_ = netConn.Close() // prevent fd leak if STOMP handshake fails
		return fmt.Errorf("activemq: dial %s: %w", mc.addr, err)
	}

	mc.mu.Lock()
	if mc.closed {
		mc.mu.Unlock()
		_ = netConn.Close()
		return fmt.Errorf("activemq: the connection to %s was not installed because the queue is closed", mc.addr)
	}
	oldConn := mc.conn
	mc.conn = stompConn
	mc.netConn = netConn
	mc.generation++
	mc.mu.Unlock()

	if oldConn != nil {
		_ = oldConn.Disconnect()
	}
	return nil
}

// getConn returns the current connection under a read lock.
func (mc *managedConn) getConn() *stomp.Conn {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.conn
}

// getConnAndGeneration returns the current connection and its generation under
// a read lock. Consumers use this to detect whether a closed subscription came
// from a stale, intentionally retired connection.
func (mc *managedConn) getConnAndGeneration() (*stomp.Conn, uint64) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.conn, mc.generation
}

// shutdown signals all consumer goroutines to stop. Safe to call more than
// once.
func (mc *managedConn) shutdown() {
	mc.mu.Lock()
	mc.closed = true
	mc.mu.Unlock()

	mc.doneOnce.Do(func() { close(mc.done) })
}

// isShuttingDown returns true after shutdown has been called.
func (mc *managedConn) isShuttingDown() bool {
	select {
	case <-mc.done:
		return true
	default:
		return false
	}
}

// closeWithin stops the consumer goroutines and disconnects from the broker
// inside ctx. It is safe to call more than once.
func (mc *managedConn) closeWithin(ctx context.Context) error {

	mc.closeOnce.Do(func() { mc.closeErr = mc.disconnectWithin(ctx) })
	return mc.closeErr
}

// disconnectWithin ends the current connection inside ctx. go-stomp waits
// longer for a disconnect receipt than the whole shutdown deadline, so the
// socket is closed by force when ctx expires first.
//
// The connection itself is kept, so a late Enqueue reports a send error.
func (mc *managedConn) disconnectWithin(ctx context.Context) error {

	mc.shutdown()

	mc.mu.RLock()
	conn, netConn := mc.conn, mc.netConn
	mc.mu.RUnlock()

	if conn == nil {
		return nil
	}

	result := make(chan error, 1)
	go func() { result <- conn.Disconnect() }()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
	}

	if netConn != nil {
		_ = netConn.Close()
	}
	<-result

	return fmt.Errorf("activemq: the shutdown deadline passed, so the connection to %s was closed by force: %w",
		mc.addr, ctx.Err())
}

// reconnectWithBackoff attempts to re-establish the connection, retrying up
// to maxAttempts times with exponential backoff capped at maxBackoff. Pass 0
// for unlimited retries (used by long-lived consumers). Returns an error if
// all attempts are exhausted or if shutdown is signalled.
func (mc *managedConn) reconnectWithBackoff(context string, maxAttempts int) error {
	mc.reconnectMu.Lock()
	defer mc.reconnectMu.Unlock()

	logger := log.GetLogger()
	backoff := initialBackoff
	attempt := 0

	for {
		if mc.isShuttingDown() {
			return fmt.Errorf("activemq: reconnect aborted, shutting down (%s)", context)
		}

		attempt++
		if maxAttempts > 0 && attempt > maxAttempts {
			return fmt.Errorf("activemq: exhausted %d reconnect attempts (%s)", maxAttempts, context)
		}

		logger.Error(fmt.Sprintf(
			"activemq: connection lost (%s), reconnecting in %s (attempt %d)…",
			context, backoff, attempt,
		))

		timer := time.NewTimer(backoff)
		select {
		case <-mc.done:
			timer.Stop()
			return fmt.Errorf("activemq: reconnect aborted, shutting down (%s)", context)
		case <-timer.C:
		}

		if mc.isShuttingDown() {
			return fmt.Errorf("activemq: reconnect aborted, shutting down (%s)", context)
		}

		if err := mc.dial(); err != nil {
			logger.Error(fmt.Sprintf("activemq: reconnect failed: %v", err))
			backoff *= backoffMultiplier
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		logger.Info(fmt.Sprintf("activemq: reconnected successfully (%s)", context))
		return nil
	}
}

// subscribeCurrent subscribes on the current live connection and returns the
// subscription together with the generation the subscription belongs to.
//
// The subscription acknowledges each message on its own, so the broker keeps a
// message until the work it describes is done.
func (mc *managedConn) subscribeCurrent(destination string) (*stomp.Subscription, uint64, error) {
	conn, generation := mc.getConnAndGeneration()
	if conn == nil {
		return nil, generation, fmt.Errorf("activemq: no active connection available for subscription")
	}

	sub, err := conn.Subscribe(destination, stomp.AckClientIndividual)
	if err != nil {
		return nil, generation, err
	}

	return sub, generation, nil
}

// -----------------------------------------------------------------------
// ProfileQueue
// -----------------------------------------------------------------------

// ProfileQueue is the ActiveMQ-backed ProfileUnificationQueue.
type ProfileQueue struct {
	mc          *managedConn
	destination string
}

func NewProfileQueue(addr, username, password, destination string, tlsCfg config.TLSConfig) (*ProfileQueue, error) {
	mc, err := newManagedConn(addr, username, password, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("activemq: failed to connect for profile queue: %w", err)
	}
	return &ProfileQueue{mc: mc, destination: destination}, nil
}

// Enqueue marshals the profile to JSON and sends it to ActiveMQ.
//
// Retry policy: if the initial send fails (typically a dead connection),
// Enqueue reconnects once and retries a single time. This is intentionally
// limited compared to the unlimited-retry loop in Start, because Enqueue is
// called in the request path where blocking indefinitely is unacceptable.
// Callers that need stronger delivery guarantees should persist the item and
// retry externally.
func (q *ProfileQueue) Enqueue(profile profileModel.Profile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("activemq: failed to marshal profile %s: %w", profile.ProfileId, err)
	}

	if err := q.mc.getConn().Send(q.destination, contentTypeJSON, data); err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: send failed for profile %s, will reconnect and retry: %v",
			profile.ProfileId, err))

		if reconnErr := q.mc.reconnectWithBackoff("profile enqueue", 1); reconnErr != nil {
			return fmt.Errorf("activemq: send failed for profile %s: %w", profile.ProfileId, reconnErr)
		}

		// Single retry after reconnect.
		if retryErr := q.mc.getConn().Send(q.destination, contentTypeJSON, data); retryErr != nil {
			return fmt.Errorf("activemq: retry send failed for profile %s: %w", profile.ProfileId, retryErr)
		}
	}
	return nil
}

// Start subscribes to the destination and reads it in its own goroutine.
//
// A message is acknowledged only after its unification is done, so a failure
// leaves the work in the queue. The consumer subscribes again to bring the
// message back, and a message that fails every attempt goes to the dead letter
// destination, where an operator can see it and the queue behind it can move.
//
// The consumer lives as long as the process. It rebuilds a lost connection
// with a growing backoff, and it stops when Close is called.
func (q *ProfileQueue) Start(handler func(profileModel.Profile) error) error {

	work := func(body []byte) error {
		var profile profileModel.Profile
		if err := json.Unmarshal(body, &profile); err != nil {
			return fmt.Errorf("%w: %v", errMalformed, err)
		}
		return handler(profile)
	}

	if err := newConsumer(q.mc, q.destination, "profile queue", work).start(); err != nil {
		return fmt.Errorf("activemq: failed to subscribe to profile queue %s: %w", q.destination, err)
	}
	return nil
}

// Close signals the consumer goroutine to stop and disconnects from ActiveMQ
// inside ctx. Safe to call more than once.
func (q *ProfileQueue) Close(ctx context.Context) error {
	return q.mc.closeWithin(ctx)
}

// -----------------------------------------------------------------------
// SchemaSyncQueue
// -----------------------------------------------------------------------

// SchemaSyncQueue is the ActiveMQ-backed SchemaSyncQueue.
type SchemaSyncQueue struct {
	mc          *managedConn
	destination string
}

func NewSchemaSyncQueue(addr, username, password, destination string, tlsCfg config.TLSConfig) (*SchemaSyncQueue, error) {
	mc, err := newManagedConn(addr, username, password, tlsCfg)
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

	if err := q.mc.getConn().Send(q.destination, contentTypeJSON, data); err != nil {
		log.GetLogger().Error(fmt.Sprintf(
			"activemq: send failed for schema sync tenant %s, will reconnect and retry: %v",
			sync.OrgId, err))

		if reconnErr := q.mc.reconnectWithBackoff("schema sync enqueue", 1); reconnErr != nil {
			return fmt.Errorf("activemq: send failed for schema sync tenant %s: %w", sync.OrgId, reconnErr)
		}

		if retryErr := q.mc.getConn().Send(q.destination, contentTypeJSON, data); retryErr != nil {
			return fmt.Errorf("activemq: retry send failed for schema sync tenant %s: %w", sync.OrgId, retryErr)
		}
	}
	return nil
}

// Start subscribes to the destination and reads it in its own goroutine.
// See ProfileQueue.Start for how a message is acknowledged and retried.
func (q *SchemaSyncQueue) Start(handler func(schemaModel.ProfileSchemaSync) error) error {

	work := func(body []byte) error {
		var sync schemaModel.ProfileSchemaSync
		if err := json.Unmarshal(body, &sync); err != nil {
			return fmt.Errorf("%w: %v", errMalformed, err)
		}
		return handler(sync)
	}

	if err := newConsumer(q.mc, q.destination, "schema sync queue", work).start(); err != nil {
		return fmt.Errorf("activemq: failed to subscribe to schema sync queue %s: %w", q.destination, err)
	}
	return nil
}

// Close signals the consumer goroutine to stop and disconnects from ActiveMQ
// inside ctx. Safe to call more than once.
func (q *SchemaSyncQueue) Close(ctx context.Context) error {
	return q.mc.closeWithin(ctx)
}
