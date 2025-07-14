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
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/handler"
	"net/http"
	"strings"
)

type ProfileSchemaService struct {
	handler *handler.ProfileSchemaHandler
}

func NewProfileSchemaService(mux *http.ServeMux, apiBasePath string) *ProfileSchemaService {

	instance := &ProfileSchemaService{
		handler: handler.NewProfileSchemaHandler(),
	}
	instance.RegisterRoutes(mux, apiBasePath)

	return instance
}

func (s *ProfileSchemaService) RegisterRoutes(mux *http.ServeMux, apiBasePath string) {
	mux.HandleFunc(apiBasePath+"/profile-schema", s.routeSchemaCollection)
	mux.HandleFunc(apiBasePath+"/profile-schema/", s.routeSchemaScopedOrAttribute)
}

func (s *ProfileSchemaService) routeSchemaCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handler.GetProfileSchema(w, r)
	case http.MethodDelete:
		s.handler.DeleteProfileSchema(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (s *ProfileSchemaService) routeSchemaScopedOrAttribute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/profile-schema/")
	parts := strings.Split(path, "/")

	// Handle /profile-schema/sync as a separate route
	if path == "sync" {
		if r.Method == http.MethodPost {
			s.handler.SyncProfileSchema(w, r) // Call your sync handler method
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	switch len(parts) {
	case 1:
		scope := parts[0]
		switch r.Method {
		case http.MethodPost:
			s.handler.AddProfileSchemaAttributesForScope(w, r, scope)
		case http.MethodGet:
			s.handler.GetProfileSchemaAttributeForScope(w, r, scope)
		case http.MethodDelete:
			s.handler.DeleteProfileSchemaAttributeForScope(w, r, scope)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case 2:
		scope, attrID := parts[0], parts[1]
		switch r.Method {
		case http.MethodGet:
			s.handler.GetProfileSchemaAttributeById(w, r, scope, attrID)
		case http.MethodPut:
			s.handler.PatchProfileSchemaAttributeById(w, r, scope, attrID)
		case http.MethodDelete:
			s.handler.DeleteProfileSchemaAttributeById(w, r, scope, attrID)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}
