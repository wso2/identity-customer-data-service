/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
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
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/handler"
	"net/http"
	"strings"
)

type UnificationRulesService struct {
	unificationRulesHandler *handler.UnificationRulesHandler
}

func NewUnificationRulesService() *UnificationRulesService {
	return &UnificationRulesService{
		unificationRulesHandler: handler.NewUnificationRulesHandler(),
	}
}

// Route handles all tenant-aware unification rules endpoints
func (s *UnificationRulesService) Route(w http.ResponseWriter, r *http.Request) {

	path := strings.TrimPrefix(r.URL.Path, "/default/iam-cdm/v1.0") // Trim fixed base path
	path = strings.TrimSuffix(path, "/")
	method := r.Method

	switch {
	case method == http.MethodPost && path == "/unification-rules":
		s.unificationRulesHandler.AddUnificationRule(w, r)

	case method == http.MethodGet && path == "/unification-rules":
		s.unificationRulesHandler.GetUnificationRules(w, r)

	case method == http.MethodGet && strings.HasPrefix(path, "/unification-rules/"):
		s.unificationRulesHandler.GetUnificationRule(w, r)

	case method == http.MethodPatch && strings.HasPrefix(path, "/unification-rules/"):
		s.unificationRulesHandler.PatchUnificationRule(w, r)

	case method == http.MethodDelete && strings.HasPrefix(path, "/unification-rules/"):
		s.unificationRulesHandler.DeleteUnificationRule(w, r)

	default:
		http.NotFound(w, r)
	}
}
