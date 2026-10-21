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
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func token(payload string) string {
	return "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".signature"
}

// The audience claim is a string in some tokens and an array in others, and
// the check CDS performs accepts both, so reading it has to as well.
func TestJWTAudiences(t *testing.T) {

	cases := []struct {
		name  string
		token string
		want  []string
	}{
		{"a single audience", token(`{"aud":"iam-cds"}`), []string{"iam-cds"}},
		{"several audiences", token(`{"aud":["other","iam-cds"]}`), []string{"other", "iam-cds"}},
		{"no audience", token(`{"iss":"https://is.example.com"}`), nil},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			claims, err := jwtClaims(test.token)
			if err != nil {
				t.Fatalf("jwtClaims: %v", err)
			}
			got := claims.audiences()
			if len(got) != len(test.want) {
				t.Fatalf("audiences = %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Errorf("audience %d = %q, want %q", i, got[i], test.want[i])
				}
			}
		})
	}
}

// An opaque token is introspected rather than parsed, so failing to decode one
// has to be distinguishable from a JWT that is missing its audience.
func TestJWTClaimsRejectsAnOpaqueToken(t *testing.T) {

	if _, err := jwtClaims("f4b8c1de-0000-opaque"); err == nil {
		t.Fatal("expected an error for a token that is not a JWT")
	}
}

func TestSecretsRoundTrip(t *testing.T) {

	cdsHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cdsHome, confDir), 0o755); err != nil {
		t.Fatalf("could not stage the configuration directory: %v", err)
	}

	if err := saveSecrets(cdsHome, map[string]string{"AUTH_SERVER_CLIENT_SECRET": "first"}); err != nil {
		t.Fatalf("saveSecrets: %v", err)
	}
	// A second save adds without dropping what the first wrote, which is what
	// lets the two phases of enablement each contribute their own secrets.
	if err := saveSecrets(cdsHome, map[string]string{"INTROSPECTION_CLIENT_SECRET": "second"}); err != nil {
		t.Fatalf("saveSecrets: %v", err)
	}

	secrets, err := loadSecrets(cdsHome)
	if err != nil {
		t.Fatalf("loadSecrets: %v", err)
	}
	if secrets["AUTH_SERVER_CLIENT_SECRET"] != "first" {
		t.Errorf("the first secret was lost: %q", secrets["AUTH_SERVER_CLIENT_SECRET"])
	}
	if secrets["INTROSPECTION_CLIENT_SECRET"] != "second" {
		t.Errorf("the second secret was not stored: %q", secrets["INTROSPECTION_CLIENT_SECRET"])
	}

	// Credentials on disk are readable by the account that runs CDS, nobody else.
	info, err := os.Stat(filepath.Join(cdsHome, secretsFile))
	if err != nil {
		t.Fatalf("could not stat the secrets file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("secrets file mode = %o, want 600", mode)
	}
}

func TestLoadSecretsWithoutAFile(t *testing.T) {

	// An installation whose secrets come from a secret manager has no file,
	// and that is not a failure.
	secrets, err := loadSecrets(t.TempDir())
	if err != nil {
		t.Fatalf("loadSecrets: %v", err)
	}
	if len(secrets) != 0 {
		t.Errorf("expected no secrets, got %d", len(secrets))
	}
}

func TestIntegrationRoundTrip(t *testing.T) {

	cdsHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cdsHome, confDir), 0o755); err != nil {
		t.Fatalf("could not stage the configuration directory: %v", err)
	}

	if _, err := loadIntegration(cdsHome); err == nil {
		t.Fatal("an installation that was never enabled should report that")
	}

	saved := &integration{
		ISHome:           "/opt/wso2is",
		ISVersion:        "7.4.0",
		CDSVersion:       "1.2.0",
		ExtensionVersion: "0.9.0",
		Extensions:       []string{"org.wso2.identity.customer.data.service.client-0.9.0.jar"},
		Organization:     defaultOrganization,
		RestartPending:   true,
	}
	if err := saveIntegration(cdsHome, saved); err != nil {
		t.Fatalf("saveIntegration: %v", err)
	}

	loaded, err := loadIntegration(cdsHome)
	if err != nil {
		t.Fatalf("loadIntegration: %v", err)
	}
	if loaded.ISHome != saved.ISHome || loaded.ISVersion != saved.ISVersion {
		t.Errorf("the recorded Identity Server did not survive: %+v", loaded)
	}
	if !loaded.RestartPending {
		t.Error("a pending restart was not recorded, so status would claim the extensions are active")
	}
	if len(loaded.Extensions) != 1 {
		t.Errorf("the installed bundles were not recorded, so disable would leave them behind")
	}

	if err := clearIntegration(cdsHome); err != nil {
		t.Fatalf("clearIntegration: %v", err)
	}
	// Disable has to converge on removed from any state, so clearing twice
	// is not an error.
	if err := clearIntegration(cdsHome); err != nil {
		t.Fatalf("clearIntegration is not idempotent: %v", err)
	}
}
