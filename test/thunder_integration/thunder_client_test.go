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

// Package thunderintegration exercises internal/identity_provider/thunder
// against a real, running WSO2 Thunder instance (started via testcontainers
// in TestMain), rather than the httptest-mocked server used by
// internal/identity_provider/thunder's own unit tests. Requires Docker.
package thunderintegration

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/identity_provider/thunder"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/test/setup"
)

var thunderClient *thunder.Client

func TestMain(m *testing.M) {
	if err := log.Init("DEBUG"); err != nil {
		panic(err)
	}

	ctx := context.Background()
	tc, err := setup.SetupTestThunder(ctx)
	if err != nil {
		panic("Failed to start test Thunder container: " + err.Error())
	}
	defer tc.Container.Terminate(ctx)

	// Thunder serves HTTPS with a self-signed certificate baked into the
	// image; skipping verification is acceptable only for this ephemeral
	// test container, never in production (see NewClientWithHTTPClient's doc).
	insecureClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

	thunderClient = thunder.NewClientWithHTTPClient(config.ThunderConfig{
		Host:                  tc.BaseURL,
		ClientID:              tc.ClientID,
		ClientSecret:          tc.ClientSecret,
		TokenEndpoint:         "/oauth2/token",
		IntrospectionEndpoint: "/oauth2/introspect",
		JWKSEndpoint:          "/oauth2/jwks",
		ApplicationsEndpoint:  "/applications",
	}, insecureClient)

	os.Exit(m.Run())
}

func TestFetchToken_AgainstRealThunder(t *testing.T) {
	token, err := thunderClient.FetchToken("unused")
	if err != nil {
		t.Fatalf("FetchToken against a real Thunder instance failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty access token")
	}
}

func TestIntrospectToken_AgainstRealThunder(t *testing.T) {
	token, err := thunderClient.FetchToken("unused")
	if err != nil {
		t.Fatalf("FetchToken failed: %v", err)
	}

	claims, err := thunderClient.IntrospectToken(token, "unused")
	if err != nil {
		t.Fatalf("IntrospectToken against a real Thunder instance failed: %v", err)
	}
	if active, _ := claims["active"].(bool); !active {
		t.Errorf("expected active=true from a freshly issued token, got %v", claims["active"])
	}
	if claims["client_id"] != setup.ThunderTestClientID {
		t.Errorf("expected client_id=%s, got %v", setup.ThunderTestClientID, claims["client_id"])
	}
}

func TestVerifyJWT_AgainstRealThunder(t *testing.T) {
	token, err := thunderClient.FetchToken("unused")
	if err != nil {
		t.Fatalf("FetchToken failed: %v", err)
	}

	claims, err := thunderClient.VerifyJWT(token)
	if err != nil {
		t.Fatalf("VerifyJWT (real JWKS-based signature verification) failed: %v", err)
	}
	if claims["client_id"] != setup.ThunderTestClientID {
		t.Errorf("expected client_id=%s in verified claims, got %v", setup.ThunderTestClientID, claims["client_id"])
	}

	// Thunder client_credentials tokens carry ouId/ouHandle claims (unlike
	// the introspection response, which does not) - see
	// docs/guides/identity-providers.md for what this does and doesn't buy
	// CDS today.
	if claims["ouHandle"] != "default" {
		t.Errorf("expected ouHandle=default in the JWT claims, got %v", claims["ouHandle"])
	}
}

// TestFetchApplicationIdentifier_AgainstRealThunder documents (not just
// works around) a real Thunder limitation as of v0.48.0: role assignments
// only accept user/group assignees, not applications, so a
// client_credentials token can never be granted access to the /applications
// management API - it is unconditionally forbidden, regardless of the scope
// requested. See docs/guides/identity-providers.md.
func TestFetchApplicationIdentifier_AgainstRealThunder(t *testing.T) {
	_, err := thunderClient.FetchApplicationIdentifier(setup.ThunderTestClientID, "unused")
	if err == nil {
		t.Fatal("expected FetchApplicationIdentifier to fail against Thunder v0.48.0 (known 403 limitation), got nil error")
	}
}
