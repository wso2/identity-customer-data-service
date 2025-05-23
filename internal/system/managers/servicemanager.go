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
	"github.com/wso2/identity-customer-data-service/internal/system/services"
	"net/http"
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

	// Register the unification rules service.
	services.NewUnificationRulesService(sm.mux, apiBasePath)
	services.NewEnrichmentRulesService(sm.mux, apiBasePath)
	services.NewEventService(sm.mux, apiBasePath)
	services.NewProfileService(sm.mux, apiBasePath)
	services.NewEventStreamIdService(sm.mux, apiBasePath)
	services.NewConsentCategoryService(sm.mux, apiBasePath)
	return nil
}
