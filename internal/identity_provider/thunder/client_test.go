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

package thunder

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	c := &Client{
		BaseURL:    strings.TrimPrefix(server.URL, "https://"),
		HTTPClient: server.Client(),
		cfg: config.ThunderConfig{
			ClientID:              "test-client",
			ClientSecret:          "test-secret",
			TokenEndpoint:         "/oauth2/token",
			IntrospectionEndpoint: "/oauth2/introspect",
			JWKSEndpoint:          "/oauth2/jwks",
			ApplicationsEndpoint:  "/applications",
		},
	}
	return c, server
}

func TestFetchToken(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("grant_type"); got != "client_credentials" {
			t.Errorf("expected grant_type=client_credentials, got %q", got)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "test-client" || pass != "test-secret" {
			t.Errorf("unexpected basic auth: %s/%s (ok=%v)", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok-123"})
	}))

	token, err := c.FetchToken("carbon.super")
	if err != nil {
		t.Fatalf("FetchToken failed: %v", err)
	}
	if token != "tok-123" {
		t.Errorf("expected tok-123, got %q", token)
	}
}

func TestFetchToken_NonOKStatus(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))

	if _, err := c.FetchToken(""); err == nil {
		t.Fatal("expected an error for a non-200 token response, got nil")
	}
}

func TestIntrospectToken(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/introspect" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"active":    true,
			"client_id": "abc123",
			"scope":     "internal_cds_profile_view",
		})
	}))

	claims, err := c.IntrospectToken("some-token", "unused-org")
	if err != nil {
		t.Fatalf("IntrospectToken failed: %v", err)
	}
	if active, _ := claims["active"].(bool); !active {
		t.Errorf("expected active=true, got %v", claims["active"])
	}
	if claims["client_id"] != "abc123" {
		t.Errorf("expected client_id=abc123, got %v", claims["client_id"])
	}
}

func TestFetchApplicationIdentifier_SinglePageMatch(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(thunderApplicationsPage{
			TotalResults: 2,
			Applications: []thunderApplication{
				{ID: "id-1", Name: "App One", ClientId: "other-client"},
				{ID: "id-2", Name: "Target App", ClientId: "target-client"},
			},
		})
	}))

	result, err := c.FetchApplicationIdentifier("target-client", "unused-org")
	if err != nil {
		t.Fatalf("FetchApplicationIdentifier failed: %v", err)
	}
	if len(result.Applications) != 1 || result.Applications[0].ClientId != "target-client" {
		t.Fatalf("expected exactly one match for target-client, got %+v", result.Applications)
	}
}

func TestFetchApplicationIdentifier_NoMatch(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
			return
		}
		_ = json.NewEncoder(w).Encode(thunderApplicationsPage{
			TotalResults: 1,
			Applications: []thunderApplication{{ID: "id-1", Name: "App One", ClientId: "other-client"}},
		})
	}))

	result, err := c.FetchApplicationIdentifier("nonexistent-client", "unused-org")
	if err != nil {
		t.Fatalf("FetchApplicationIdentifier failed: %v", err)
	}
	if len(result.Applications) != 0 {
		t.Fatalf("expected no matches, got %+v", result.Applications)
	}
}

func TestFetchApplicationIdentifier_Pagination(t *testing.T) {
	// Page 0 is a full page (== applicationsPageLimit) with no match, forcing
	// the client to request page 1, which contains the match.
	fullPage := make([]thunderApplication, applicationsPageLimit)
	for i := range fullPage {
		fullPage[i] = thunderApplication{ID: fmt.Sprintf("id-%d", i), ClientId: fmt.Sprintf("app-%d", i)}
	}

	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
			return
		}
		offset := r.URL.Query().Get("offset")
		w.Header().Set("Content-Type", "application/json")
		if offset == "0" {
			_ = json.NewEncoder(w).Encode(thunderApplicationsPage{Applications: fullPage})
			return
		}
		_ = json.NewEncoder(w).Encode(thunderApplicationsPage{
			Applications: []thunderApplication{{ID: "id-page2", ClientId: "target-client"}},
		})
	}))

	result, err := c.FetchApplicationIdentifier("target-client", "unused-org")
	if err != nil {
		t.Fatalf("FetchApplicationIdentifier failed: %v", err)
	}
	if len(result.Applications) != 1 || result.Applications[0].ID != "id-page2" {
		t.Fatalf("expected the page-2 match, got %+v", result.Applications)
	}
}

// jwksTestFixture generates an RSA keypair and the JWKS document for it, plus
// a helper to sign tokens with it.
type jwksTestFixture struct {
	key *rsa.PrivateKey
	kid string
}

func newJWKSTestFixture(t *testing.T) *jwksTestFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return &jwksTestFixture{key: key, kid: "test-kid-1"}
}

func (f *jwksTestFixture) jwksJSON() []byte {
	n := base64.RawURLEncoding.EncodeToString(f.key.PublicKey.N.Bytes())
	eBytes := []byte{byte(f.key.PublicKey.E >> 16), byte(f.key.PublicKey.E >> 8), byte(f.key.PublicKey.E)}
	e := base64.RawURLEncoding.EncodeToString(eBytes)
	doc := map[string]interface{}{
		"keys": []map[string]string{
			{"kty": "RSA", "kid": f.kid, "alg": "RS256", "n": n, "e": e},
		},
	}
	b, _ := json.Marshal(doc)
	return b
}

func (f *jwksTestFixture) sign(t *testing.T, claims jwt.MapClaims, kid string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(f.key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestVerifyJWT_Valid(t *testing.T) {
	fixture := newJWKSTestFixture(t)
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture.jwksJSON())
	}))

	token := fixture.sign(t, jwt.MapClaims{"sub": "user1", "exp": time.Now().Add(time.Hour).Unix()}, fixture.kid)

	claims, err := c.VerifyJWT(token)
	if err != nil {
		t.Fatalf("VerifyJWT failed for a validly signed token: %v", err)
	}
	if claims["sub"] != "user1" {
		t.Errorf("expected sub=user1, got %v", claims["sub"])
	}
}

func TestVerifyJWT_Expired(t *testing.T) {
	fixture := newJWKSTestFixture(t)
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture.jwksJSON())
	}))

	token := fixture.sign(t, jwt.MapClaims{"sub": "user1", "exp": time.Now().Add(-time.Hour).Unix()}, fixture.kid)

	if _, err := c.VerifyJWT(token); err == nil {
		t.Fatal("expected an error for an expired token, got nil")
	}
}

func TestVerifyJWT_WrongKid(t *testing.T) {
	fixture := newJWKSTestFixture(t)
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture.jwksJSON())
	}))

	token := fixture.sign(t, jwt.MapClaims{"sub": "user1", "exp": time.Now().Add(time.Hour).Unix()}, "some-other-kid")

	if _, err := c.VerifyJWT(token); err == nil {
		t.Fatal("expected an error when the token's kid isn't in the JWKS, got nil")
	}
}

func TestVerifyJWT_TamperedSignature(t *testing.T) {
	fixture := newJWKSTestFixture(t)
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture.jwksJSON())
	}))

	token := fixture.sign(t, jwt.MapClaims{"sub": "user1", "exp": time.Now().Add(time.Hour).Unix()}, fixture.kid)
	tampered := token[:len(token)-2] + "xx"

	if _, err := c.VerifyJWT(tampered); err == nil {
		t.Fatal("expected an error for a tampered signature, got nil")
	}
}
