/*
 * Copyright (c) 2025-2026, WSO2 LLC. (http://www.wso2.com).
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
	"fmt"

	"github.com/wso2/identity-customer-data-service/internal/identity_provider"
	"github.com/wso2/identity-customer-data-service/internal/identity_provider/thunder"
	"github.com/wso2/identity-customer-data-service/internal/identity_provider/wso2is"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// NewIdentityClient constructs the identity_provider.Client selected by
// auth_server.provider ("wso2is", the default, or "thunder"). Assumes
// ValidateAuthServerConfig was already called at startup; an
// empty/unrecognized value falls back to wso2is (logged) rather than
// crashing mid-request.
func NewIdentityClient(cfg config.Config) identity_provider.Client {
	switch cfg.AuthServer.Provider {
	case constants.IdentityProviderThunder:
		return thunder.NewClient(cfg)
	case "", constants.IdentityProviderWSO2IS:
		return wso2is.NewClient(cfg)
	default:
		log.GetLogger().Warn(fmt.Sprintf(
			"Unknown auth_server.provider %q; defaulting to %q", cfg.AuthServer.Provider, constants.IdentityProviderWSO2IS))
		return wso2is.NewClient(cfg)
	}
}

// ValidateAuthServerConfig fails fast on an unrecognized auth_server.provider
// value. Call once at startup (cmd/server/main.go) before serving traffic.
func ValidateAuthServerConfig(cfg config.Config) error {
	switch cfg.AuthServer.Provider {
	case "", constants.IdentityProviderWSO2IS, constants.IdentityProviderThunder:
		return nil
	default:
		return fmt.Errorf("invalid auth_server.provider %q (expected %q or %q)",
			cfg.AuthServer.Provider, constants.IdentityProviderWSO2IS, constants.IdentityProviderThunder)
	}
}
