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
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/handler"
)

type EventStreamIdService struct {
	apiKeyHandler *handler.EventStreamIdHandler
}

func NewEventStreamIdService(mux *http.ServeMux, apiBasePath string) *EventStreamIdService {
	instance := &EventStreamIdService{
		apiKeyHandler: handler.NewEventStreamIdHandler(),
	}
	instance.RegisterRoutes(mux, apiBasePath)
	return instance
}

func (s *EventStreamIdService) RegisterRoutes(mux *http.ServeMux, apiBasePath string) {

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		// Add or list API keys: POST or GET /applications/{app-uuid}/api-keys
		case r.Method == http.MethodPost && strings.Contains(path, "/event-stream-ids"):
			s.apiKeyHandler.AddEventStreamId(w, r)
		case r.Method == http.MethodGet && strings.Contains(path, "/event-stream-ids/"):
			s.apiKeyHandler.GetEventStreamId(w, r)
		case r.Method == http.MethodGet && strings.Contains(path, "/event-stream-ids"):
			s.apiKeyHandler.GetEventStreamIdPerApp(w, r)
		// Rotate: PUT /applications/{app-uuid}/api-keys/{key-id}/rotate
		case r.Method == http.MethodPut && strings.Contains(path, "/event-stream-ids/") && strings.HasSuffix(path, "/rotate"):
			s.apiKeyHandler.RotateEventStreamId(w, r)
		// Revoke: PUT /applications/{app-uuid}/api-keys/{key-id}/revoke
		case r.Method == http.MethodPut && strings.Contains(path, "/event-stream-ids/") && strings.HasSuffix(path, "/revoke"):
			s.apiKeyHandler.RevokeEventStreamId(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}
