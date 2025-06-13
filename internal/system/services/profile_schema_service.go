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
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/handler"
	"net/http"
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
	mux.HandleFunc(fmt.Sprintf("GET %s/profile-schema", apiBasePath), s.handler.GetProfileSchema)
	mux.HandleFunc(fmt.Sprintf("DELETE %s/profile-schema", apiBasePath), s.handler.DeleteProfileSchema)
	mux.HandleFunc(fmt.Sprintf("POST %s/profile-schema", apiBasePath), s.handler.AddProfileSchemaAttribute)
	mux.HandleFunc(fmt.Sprintf("GET %s/profile-schema/", apiBasePath), s.handler.GetProfileSchemaAttribute)
	mux.HandleFunc(fmt.Sprintf("PATCH %s/profile-schema/", apiBasePath), s.handler.PatchProfileSchemaAttribute)
	mux.HandleFunc(fmt.Sprintf("DELETE %s/profile-schema/", apiBasePath), s.handler.DeleteProfileSchemaAttribute)
}
