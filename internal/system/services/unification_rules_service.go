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

package services

import (
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/handler"
	"net/http"
)

type UnificationRulesService struct {
	applicationHandler *handler.UnificationRulesHandler
}

func NewUnificationRulesService(mux *http.ServeMux, apiBasePath string) *UnificationRulesService {

	instance := &UnificationRulesService{
		applicationHandler: handler.NewUnificationRulesHandler(),
	}
	instance.RegisterRoutes(mux, apiBasePath)

	return instance
}

func (s *UnificationRulesService) RegisterRoutes(mux *http.ServeMux, apiBasePath string) {

	mux.HandleFunc(fmt.Sprintf("POST %s/unification-rules", apiBasePath), s.applicationHandler.AddUnificationRule)
	mux.HandleFunc(fmt.Sprintf("GET %s/unification-rules", apiBasePath), s.applicationHandler.GetUnificationRules)
	mux.HandleFunc(fmt.Sprintf("GET %s/unification-rules/", apiBasePath), s.applicationHandler.GetUnificationRule)
	mux.HandleFunc(fmt.Sprintf("PATCH %s/unification-rules/", apiBasePath), s.applicationHandler.PatchUnificationRule)
	mux.HandleFunc(fmt.Sprintf("DELETE %s/unification-rules/", apiBasePath), s.applicationHandler.DeleteUnificationRule)
}
