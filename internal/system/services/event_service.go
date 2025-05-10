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
	"github.com/wso2/identity-customer-data-service/internal/events/handler"
	"net/http"
)

type EventService struct {
	eventHandler *handler.EventHandler
}

func NewEventService(mux *http.ServeMux, apiBasePath string) *EventService {

	instance := &EventService{
		eventHandler: handler.NewEventHandler(),
	}
	instance.RegisterRoutes(mux, apiBasePath)

	return instance
}

func (s *EventService) RegisterRoutes(mux *http.ServeMux, apiBasePath string) {

	mux.HandleFunc(fmt.Sprintf("POST %s/events", apiBasePath), s.eventHandler.AddEvent)
	mux.HandleFunc(fmt.Sprintf("GET %s/events", apiBasePath), s.eventHandler.GetEvents)
	mux.HandleFunc(fmt.Sprintf("GET %s/events/write-key/", apiBasePath), s.eventHandler.GetWriteKey)
	mux.HandleFunc(fmt.Sprintf("GET %s/events/", apiBasePath), s.eventHandler.GetEvent)
}
