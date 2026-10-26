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

package systemapp

import (
	"strings"
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

const usableKey = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE="

// A deployment that is provisioned for stores nothing and seals nothing, so it
// must not be asked for a key it has no use for.
func TestValidateConfigIgnoresADeploymentThatDoesNotProvision(t *testing.T) {

	err := ValidateConfig(config.AuthServerConfig{
		SystemApplication: config.SystemApplicationConfig{AutoProvision: false},
	})
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func TestValidateConfigRefusesProvisioningWithoutAUsableKey(t *testing.T) {

	cases := map[string]string{
		"missing":      "",
		"not base64":   "not-base64-$$$",
		"wrong length": "c2hvcnQ=",
	}

	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateConfig(config.AuthServerConfig{
				AdminUsername: "admin",
				AdminPassword: "admin",
				SystemApplication: config.SystemApplicationConfig{
					AutoProvision: true,
					EncryptionKey: key,
				},
			})
			if err == nil {
				t.Fatalf("expected a %s key to stop the server", name)
			}
			if !strings.Contains(err.Error(), "encryption_key") {
				t.Fatalf("error does not name the setting to fix: %v", err)
			}
		})
	}
}

// Provisioning runs as the administrator, because the organisation has no
// application to get a token with until it has. Without those credentials it
// cannot do anything, and finding that out at the first enable is too late.
func TestValidateConfigRefusesProvisioningWithoutAdministratorCredentials(t *testing.T) {

	err := ValidateConfig(config.AuthServerConfig{
		AdminUsername: "admin",
		SystemApplication: config.SystemApplicationConfig{
			AutoProvision: true,
			EncryptionKey: usableKey,
		},
	})
	if err == nil {
		t.Fatal("expected a missing administrator password to stop the server")
	}
	if !strings.Contains(err.Error(), "admin_password") {
		t.Fatalf("error does not name the setting to fix: %v", err)
	}
}

func TestValidateConfigAcceptsAProvisioningDeployment(t *testing.T) {

	err := ValidateConfig(config.AuthServerConfig{
		AdminUsername: "admin",
		AdminPassword: "admin",
		SystemApplication: config.SystemApplicationConfig{
			AutoProvision: true,
			EncryptionKey: usableKey,
		},
	})
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

// The name is derived rather than stored, so it has to be the same string on
// every run: an application is otherwise created afresh each time instead of
// being adopted.
func TestSystemApplicationNameIsDerivedAndPrefixed(t *testing.T) {

	named := config.SystemApplicationConfig{NamePrefix: "Acme CDS"}
	if got := named.SystemApplicationName("acme"); got != "Acme CDS - acme" {
		t.Fatalf("named %q, want %q", got, "Acme CDS - acme")
	}

	unnamed := config.SystemApplicationConfig{}
	want := config.DefaultSystemAppNamePrefix + " - acme"
	if got := unnamed.SystemApplicationName("acme"); got != want {
		t.Fatalf("named %q, want %q", got, want)
	}
}
