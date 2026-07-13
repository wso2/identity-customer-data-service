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

package client

import (
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func TestMain(m *testing.M) {
	if err := log.Init("DEBUG"); err != nil {
		panic(err)
	}
	m.Run()
}

func TestNewIdentityClient_ProviderSelection(t *testing.T) {
	tests := []struct {
		provider     string
		expectedName string
	}{
		{"", "wso2is"},
		{"wso2is", "wso2is"},
		{"thunder", "thunder"},
		{"bogus", "wso2is"}, // unrecognized falls back to wso2is with a logged warning
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := config.Config{AuthServer: config.AuthServerConfig{Provider: tt.provider}}
			c := NewIdentityClient(cfg)
			if c.Name() != tt.expectedName {
				t.Errorf("provider=%q: expected client %q, got %q", tt.provider, tt.expectedName, c.Name())
			}
		})
	}
}

func TestValidateAuthServerConfig(t *testing.T) {
	valid := []string{"", "wso2is", "thunder"}
	for _, provider := range valid {
		if err := ValidateAuthServerConfig(config.Config{AuthServer: config.AuthServerConfig{Provider: provider}}); err != nil {
			t.Errorf("expected provider %q to be valid, got error: %v", provider, err)
		}
	}

	if err := ValidateAuthServerConfig(config.Config{AuthServer: config.AuthServerConfig{Provider: "bogus"}}); err == nil {
		t.Error("expected an error for an unrecognized provider, got nil")
	}
}
