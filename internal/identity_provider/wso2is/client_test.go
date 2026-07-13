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

package wso2is

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func TestMain(m *testing.M) {
	if err := log.Init("DEBUG"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	return &Client{
		BaseURL:    strings.TrimPrefix(server.URL, "https://"),
		HTTPClient: server.Client(),
	}
}

func withAuthServerConfig(t *testing.T, cfg config.AuthServerConfig) {
	t.Helper()
	config.OverrideCDSRuntime(config.Config{AuthServer: cfg})
}

func TestFetchToken_ClientCredentials(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("grant_type"); got != "client_credentials" {
			t.Errorf("expected grant_type=client_credentials, got %q", got)
		}
		if !strings.Contains(r.URL.Path, "/t/carbon.super/oauth2/token") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "cc-token"})
	}))

	withAuthServerConfig(t, config.AuthServerConfig{
		ClientID: "client-id", ClientSecret: "client-secret",
		TokenEndpoint: "/oauth2/token", IsSystemAppGrantEnabled: false,
	})

	token, err := c.FetchToken("carbon.super")
	if err != nil {
		t.Fatalf("FetchToken failed: %v", err)
	}
	if token != "cc-token" {
		t.Errorf("expected cc-token, got %q", token)
	}
}

func TestFetchToken_SystemAppGrant(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("grant_type"); got != "system_app_grant" {
			t.Errorf("expected grant_type=system_app_grant, got %q", got)
		}
		if got := r.Form.Get("organizationHandle"); got != "my-org" {
			t.Errorf("expected organizationHandle=my-org, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "org-token"})
	}))

	withAuthServerConfig(t, config.AuthServerConfig{
		ClientID: "client-id", ClientSecret: "client-secret",
		TokenEndpoint: "/oauth2/token", IsSystemAppGrantEnabled: true,
	})

	token, err := c.FetchToken("my-org")
	if err != nil {
		t.Fatalf("FetchToken failed: %v", err)
	}
	if token != "org-token" {
		t.Errorf("expected org-token, got %q", token)
	}
}

func TestIntrospectToken(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/t/carbon.super/oauth2/introspect") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"active": true, "client_id": "abc"})
	}))

	withAuthServerConfig(t, config.AuthServerConfig{
		IntrospectionEndPoint: "/oauth2/introspect", IsSystemAppGrantEnabled: false,
	})

	claims, err := c.IntrospectToken("tok", "myorg")
	if err != nil {
		t.Fatalf("IntrospectToken failed: %v", err)
	}
	if active, _ := claims["active"].(bool); !active {
		t.Errorf("expected active=true, got %v", claims["active"])
	}
}

func TestFetchApplicationIdentifier(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth2/token") {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
			return
		}
		filter := r.URL.Query().Get("filter")
		if !strings.Contains(filter, "clientId eq my-client") {
			t.Errorf("unexpected filter: %s", filter)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"totalResults": 1, "count": 1,
			"applications": []map[string]string{{"id": "1", "name": "App", "clientId": "my-client"}},
		})
	}))

	withAuthServerConfig(t, config.AuthServerConfig{
		TokenEndpoint: "/oauth2/token", IsSystemAppGrantEnabled: false,
	})

	result, err := c.FetchApplicationIdentifier("my-client", "myorg")
	if err != nil {
		t.Fatalf("FetchApplicationIdentifier failed: %v", err)
	}
	if len(result.Applications) != 1 {
		t.Fatalf("expected 1 application, got %d", len(result.Applications))
	}
}

func TestVerifyJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	kid := "test-kid"

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/jwks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		n := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
		eBytes := []byte{byte(key.PublicKey.E >> 16), byte(key.PublicKey.E >> 8), byte(key.PublicKey.E)}
		e := base64.RawURLEncoding.EncodeToString(eBytes)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"keys": []map[string]string{{"kty": "RSA", "kid": kid, "alg": "RS256", "n": n, "e": e}},
		})
	}))

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "user1", "exp": time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err := c.VerifyJWT(signed)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims["sub"] != "user1" {
		t.Errorf("expected sub=user1, got %v", claims["sub"])
	}
}
