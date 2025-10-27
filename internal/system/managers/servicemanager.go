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
	RegisterServices() error
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

func (sm *ServiceManager) RegisterServices() error {

	// Redirect any /api/... calls to the default tenant (covers all versions)
	utils.RewriteToDefaultTenant("/api", sm.mux, constants.DefaultTenant)

	// Create a dedicated mux for tenant-scoped routes to avoid exposing them at the root
	routesMux := http.NewServeMux()

	// Initialize services with the shared tenant routes mux so they don't create their own mux
	_ = services.NewHealthService(routesMux)
	_ = services.NewProfileService(routesMux)
	_ = services.NewProfileSchemaService(routesMux)
	_ = services.NewUnificationRulesService(routesMux)
	_ = services.NewConsentCategoryService(routesMux)

	// Single tenant dispatcher for all services; services own the versioned path (e.g., /api/v1/...)
	utils.MountTenantDispatcher(sm.mux, func(w http.ResponseWriter, r *http.Request) {
		// Normalize trailing slash prior to routing
		path := strings.TrimSuffix(r.URL.Path, "/")
		if path == "" {
			r.URL.Path = "/"
		} else {
			r.URL.Path = path
		}
		// Delegate to the tenant routes mux; it will 404 for unknown paths
		routesMux.ServeHTTP(w, r)
	})
	return nil
}
