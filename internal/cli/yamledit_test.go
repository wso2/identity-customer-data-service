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
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

const sampleConfig = `env: "${ENV}"

addr:
  host: 127.0.0.1
  port: 8900

server_url: "https://localhost:8900" # Public-facing base URL

auth_server:
  host: "localhost"
  port : 9443
  client_id: "placeholder"

tls:
  cert_dir: "/etc/certs"
`

func TestSetScalar(t *testing.T) {

	cases := []struct {
		name          string
		parent, key   string
		value         string
		wantSubstring string
	}{
		{"a string is quoted", "auth_server", "host", "is.example.com", `  host: "is.example.com"`},
		{"a number stays bare", "auth_server", "port", "9443", `  port: 9443`},
		{"a top-level scalar", "", "server_url", "https://cds.example.com", `server_url: "https://cds.example.com"`},
		{"an absolute path", "tls", "cert_dir", "/opt/cds/repository/resources/security",
			`  cert_dir: "/opt/cds/repository/resources/security"`},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := setScalar(sampleConfig, test.parent, test.key, test.value)
			if err != nil {
				t.Fatalf("setScalar: %v", err)
			}
			if !strings.Contains(got, test.wantSubstring) {
				t.Errorf("result does not contain %q:\n%s", test.wantSubstring, got)
			}
		})
	}
}

// The shipped configuration documents nearly every setting, so an edit that
// dropped the comments would cost an operator more than it gained.
func TestSetScalarKeepsComments(t *testing.T) {

	got, err := setScalar(sampleConfig, "", "server_url", "https://cds.example.com")
	if err != nil {
		t.Fatalf("setScalar: %v", err)
	}
	if !strings.Contains(got, "# Public-facing base URL") {
		t.Error("the trailing comment was dropped")
	}
	if !strings.Contains(got, `env: "${ENV}"`) {
		t.Error("an unrelated line was changed")
	}
}

// A key name that also exists under another parent must not be rewritten
// there: addr.host and auth_server.host are different settings.
func TestSetScalarIsScopedToItsParent(t *testing.T) {

	got, err := setScalar(sampleConfig, "auth_server", "host", "is.example.com")
	if err != nil {
		t.Fatalf("setScalar: %v", err)
	}
	if !strings.Contains(got, "  host: 127.0.0.1") {
		t.Error("addr.host was rewritten along with auth_server.host")
	}
}

func TestSetScalarReportsAMissingSetting(t *testing.T) {

	if _, err := setScalar(sampleConfig, "auth_server", "nonexistent", "x"); err == nil {
		t.Fatal("expected an error for a setting that is not in the file")
	}
}

// The runtime decodes these settings into typed fields, so the edited file has
// to round-trip through a YAML decoder with the types intact.
func TestEditedConfigStillDecodes(t *testing.T) {

	content := sampleConfig
	var err error
	for _, edit := range []struct{ parent, key, value string }{
		{"auth_server", "host", "is.example.com"},
		{"auth_server", "port", "9443"},
		{"tls", "cert_dir", "/opt/cds/repository/resources/security"},
		{"", "server_url", "https://cds.example.com"},
	} {
		content, err = setScalar(content, edit.parent, edit.key, edit.value)
		if err != nil {
			t.Fatalf("setScalar(%s.%s): %v", edit.parent, edit.key, err)
		}
	}

	var decoded runtimeConfig
	if err := yaml.Unmarshal([]byte(content), &decoded); err != nil {
		t.Fatalf("the edited configuration does not decode: %v\n%s", err, content)
	}
	if decoded.AuthServer.Host != "is.example.com" {
		t.Errorf("auth_server.host = %q", decoded.AuthServer.Host)
	}
	if decoded.AuthServer.Port != 9443 {
		t.Errorf("auth_server.port = %d, want 9443 as a number", decoded.AuthServer.Port)
	}
	if decoded.ServerURL != "https://cds.example.com" {
		t.Errorf("server_url = %q", decoded.ServerURL)
	}
}
