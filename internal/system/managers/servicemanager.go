/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
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

package managers

import (
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/services"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

type ServiceManagerInterface interface {
	RegisterServices(apiBasePath string) error
}

type ServiceManager struct {
	mux *http.ServeMux
}

// NewServiceManager creates a new instance of ServiceManager.
func NewServiceManager(mux *http.ServeMux) ServiceManagerInterface {

	return &ServiceManager{
		mux: mux,
	}
}

func (sm *ServiceManager) RegisterServices(apiBasePath string) error {

	utils.RewriteToDefaultTenant(apiBasePath, sm.mux, constants.DefaultTenant)

	// Create a dedicated mux for tenant-scoped routes to avoid exposing them at the root
	routesMux := http.NewServeMux()

	// Initialize services with the shared tenant routes mux so they don't create their own mux
	_ = services.NewProfileService(routesMux)
	_ = services.NewProfileSchemaService(routesMux)
	_ = services.NewUnificationRulesService(routesMux)
	_ = services.NewConsentCategoryService(routesMux)

	// Single tenant dispatcher for all services
	utils.MountTenantDispatcher(sm.mux, apiBasePath, func(w http.ResponseWriter, r *http.Request) {
		// Internal path after tenant and base path stripping
		path := strings.TrimSuffix(r.URL.Path, "/")
		if path == "" {
			path = "/"
		}
		// Delegate to the tenant routes mux; it will 404 for unknown paths
		routesMux.ServeHTTP(w, r)
	})
	return nil
}
