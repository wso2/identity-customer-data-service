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
	"github.com/wso2/identity-customer-data-service/internal/health_check/handler"
	"net/http"
	"strings"
)

// HealthService handles routing for health and readiness endpoints.
type HealthService struct {
	handler *handler.HealthHandler
}

// NewHealthService creates a new HealthService instance.
func NewHealthService() *HealthService {
	return &HealthService{
		handler: handler.NewHealthHandler(),
	}
}

// Route dispatches health and readiness requests.
func (s *HealthService) Route(w http.ResponseWriter, r *http.Request) {

	path := strings.TrimSuffix(r.URL.Path, "/")
	method := r.Method

	switch {
	case method == http.MethodGet && path == "/health":
		s.handler.HandleHealth(w, r)

	case method == http.MethodGet && path == "/ready":
		s.handler.HandleReadiness(w, r)

	default:
		http.NotFound(w, r)
	}
}
