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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeIS answers the handful of management endpoints enablement calls, and
// records what was asked of it.
type fakeIS struct {
	// authorizedAPIs are the resource ids an application already holds.
	authorizedAPIs []string

	posted  []string
	patched []string
}

func (f *fakeIS) server(t *testing.T) *httptest.Server {

	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/server/v1/applications/app-1/authorized-apis", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			f.posted = append(f.posted, "authorized-apis")
			w.WriteHeader(http.StatusCreated)
			return
		}
		out := make([]map[string]string, 0, len(f.authorizedAPIs))
		for _, id := range f.authorizedAPIs {
			out = append(out, map[string]string{"id": id})
		}
		_ = json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("/api/server/v1/applications/app-1/authorized-apis/", func(w http.ResponseWriter, r *http.Request) {
		f.patched = append(f.patched, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/server/v1/api-resources", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"apiResources": []map[string]string{{"id": "resource-1"}},
		})
	})

	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	return server
}

func (f *fakeIS) api(t *testing.T) *isAPI {

	t.Helper()
	api, err := newISAPI(f.server(t).URL, "admin", "admin", "", true)
	if err != nil {
		t.Fatalf("newISAPI: %v", err)
	}
	return api
}

// The Identity Server owns the Console's authorised APIs and refuses to patch
// them, so an authorisation that is already there must be left untouched.
func TestConsoleAuthorisationIsNeverPatched(t *testing.T) {

	fake := &fakeIS{authorizedAPIs: []string{"resource-1"}}

	added, err := fake.api(t).authorizeAPIIfAbsent("app-1", "/cds/api/v1/profiles", []string{"scope"})
	if err != nil {
		t.Fatalf("authorizeAPIIfAbsent: %v", err)
	}
	if added {
		t.Error("an authorisation that was already present was reported as added")
	}
	if len(fake.patched) != 0 {
		t.Errorf("the Console authorisation was patched: %v", fake.patched)
	}
}

// When it genuinely is absent, it is added — the Console on an Identity Server
// that has never seen CDS still needs the grant.
func TestConsoleAuthorisationIsAddedWhenAbsent(t *testing.T) {

	fake := &fakeIS{}

	added, err := fake.api(t).authorizeAPIIfAbsent("app-1", "/cds/api/v1/profiles", []string{"scope"})
	if err != nil {
		t.Fatalf("authorizeAPIIfAbsent: %v", err)
	}
	if !added {
		t.Error("an absent authorisation was not added")
	}
	if len(fake.patched) != 0 {
		t.Errorf("adding an authorisation used PATCH: %v", fake.patched)
	}
}
