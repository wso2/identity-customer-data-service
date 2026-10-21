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

package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A port answering is not proof that this installation started. Something
// else listening on the configured address — another CDS, anything — would
// otherwise let a start that failed report success, which is how a runtime
// that died in its first second once looked ready.
func TestWaitReadyDoesNotTrustAPortSomeoneElseOwns(t *testing.T) {

	// Stands in for the other process: answers /ready perfectly.
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer other.Close()

	exited := make(chan error, 1)
	exited <- errors.New("exit status 1")

	err := waitReady(other.URL, exited, 30*time.Second)
	if err == nil {
		t.Fatal("start reported ready while the runtime it started had exited")
	}
}

// A runtime that exits has to be reported at once, not after the readiness
// deadline: the operator is waiting, and the answer is already known.
func TestWaitReadyReportsAnExitImmediately(t *testing.T) {

	exited := make(chan error, 1)
	exited <- errors.New("exit status 1")

	began := time.Now()
	// An address nothing is listening on, so only the exit can end this.
	err := waitReady("https://127.0.0.1:1", exited, 30*time.Second)
	if err == nil {
		t.Fatal("expected the exit to be reported")
	}
	if waited := time.Since(began); waited > 5*time.Second {
		t.Errorf("took %s to report an exit that was already known", waited)
	}
}

func TestWaitReadySucceedsWhileTheRuntimeRuns(t *testing.T) {

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Nothing on the channel: the runtime is still going.
	if err := waitReady(server.URL, make(chan error, 1), 10*time.Second); err != nil {
		t.Fatalf("waitReady: %v", err)
	}
}

// The log level has no usable default of its own: unset is not "quiet", it is
// a parse failure that ends the process before it can say so.
func TestRuntimeDefaultsCoverTheLogLevel(t *testing.T) {

	if runtimeDefaults["LOG_LEVEL"] == "" {
		t.Error("LOG_LEVEL has no default, so a pack cannot start without one being supplied")
	}
}
