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
	"github.com/wso2/identity-customer-data-service/internal/profile/handler"
	"net/http"
)

type ProfileService struct {
	eventHandler *handler.ProfileHandler
}

func NewProfileService(mux *http.ServeMux, apiBasePath string) *ProfileService {

	instance := &ProfileService{
		eventHandler: handler.NewProfileHandler(),
	}
	instance.RegisterRoutes(mux, apiBasePath)

	return instance
}

func (s *ProfileService) RegisterRoutes(mux *http.ServeMux, apiBasePath string) {

	mux.HandleFunc(fmt.Sprintf("GET %s/profiles", apiBasePath), s.eventHandler.GetAllProfiles)
	mux.HandleFunc(fmt.Sprintf("GET %s/profiles/", apiBasePath), s.eventHandler.GetProfile)
	mux.HandleFunc(fmt.Sprintf("DELETE %s/profiles/", apiBasePath), s.eventHandler.DeleteProfile)
}
