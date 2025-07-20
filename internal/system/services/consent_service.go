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
	"github.com/wso2/identity-customer-data-service/internal/consent/handler"
	"net/http"
	"strings"
)

type ConsentCategoryService struct {
	handler *handler.ConsentCategoryHandler
}

func NewConsentCategoryService() *ConsentCategoryService {
	return &ConsentCategoryService{
		handler: handler.NewConsentCategoryHandler(),
	}
}

// Route handles tenant-aware routing for consent categories
func (s *ConsentCategoryService) Route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/default/iam-cdm/v1.0") // Trim fixed base path
	path = strings.TrimSuffix(path, "/")
	method := r.Method

	switch {
	case method == http.MethodGet && path == "/consent-categories":
		s.handler.GetAllConsentCategories(w, r)

	case method == http.MethodPost && path == "/consent-categories":
		s.handler.AddConsentCategory(w, r)

	case strings.HasPrefix(path, "/consent-categories/"):
		switch method {
		case http.MethodGet:
			s.handler.GetConsentCategory(w, r)
		case http.MethodPut:
			s.handler.UpdateConsentCategory(w, r)
		case http.MethodDelete:
			s.handler.DeleteConsentCategory(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}

	default:
		http.NotFound(w, r)
	}
}
