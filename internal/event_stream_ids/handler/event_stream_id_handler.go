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

package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/provider"
)

type EventStreamIdHandler struct{}

func NewEventStreamIdHandler() *EventStreamIdHandler {
	return &EventStreamIdHandler{}
}

func extractTenantAndApp(path string) (orgID, appID string, ok bool) {
	parts := strings.Split(path, "/")
	if len(parts) >= 9 && parts[1] == "t" && parts[6] == "applications" {
		return parts[2], parts[7], true
	} else if len(parts) >= 7 && parts[4] == "applications" {
		return "-1234", parts[5], true
	}
	return "", "", false
}

func extractEventStreamId(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 10 && parts[8] == "event-stream-ids" {
		return parts[9]
	} else if len(parts) >= 9 && parts[6] == "event-stream-ids" {
		return parts[7]
	}
	return ""
}

// AddEventStreamId handles adding a new API key
func (ah *EventStreamIdHandler) AddEventStreamId(w http.ResponseWriter, r *http.Request) {
	orgID, appID, ok := extractTenantAndApp(r.URL.Path)
	if !ok {
		http.Error(w, "invalid URL format", http.StatusBadRequest)
		return
	}
	apiKeyService := provider.NewEventStreamIdProvider().GetEventStreamIdService()
	apiKey, err := apiKeyService.CreateEventStreamId(orgID, appID)
	if err != nil {
		log.Print("failed to create api key: ", err)
		http.Error(w, "failed to create api key", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(apiKey)
	if err != nil {
		return
	}
}

// GetEventStreamId fetches either all or one API key
func (ah *EventStreamIdHandler) GetEventStreamId(w http.ResponseWriter, r *http.Request) {

	apiKeyID := extractEventStreamId(r.URL.Path)

	apiKeyService := provider.NewEventStreamIdProvider().GetEventStreamIdService()
	if apiKeyID != "" {
		log.Print("fetching api key: ", apiKeyID)
		key, err := apiKeyService.GetEventStreamId(apiKeyID)
		if err != nil || key == nil {
			http.Error(w, "api key not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(key)
		return
	}
}

// GetEventStreamIdPerApp fetches either all or one API key
func (ah *EventStreamIdHandler) GetEventStreamIdPerApp(w http.ResponseWriter, r *http.Request) {
	orgID, appID, ok := extractTenantAndApp(r.URL.Path)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "invalid URL format", http.StatusBadRequest)
		return
	}

	apiKeyService := provider.NewEventStreamIdProvider().GetEventStreamIdService()
	summary, err := apiKeyService.GetEventStreamIdPerApp(orgID, appID)
	if err != nil || summary == nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "api key not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

// RotateEventStreamId regenerates a key for a given app/key
func (ah *EventStreamIdHandler) RotateEventStreamId(w http.ResponseWriter, r *http.Request) {

	eventStreamId := extractEventStreamId(r.URL.Path)

	apiKeyService := provider.NewEventStreamIdProvider().GetEventStreamIdService()
	newKey, err := apiKeyService.RotateEventStreamId(eventStreamId)
	if err != nil {
		http.Error(w, "failed to rotate api key", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(newKey)
}

// RevokeEventStreamId disables the API key
func (ah *EventStreamIdHandler) RevokeEventStreamId(w http.ResponseWriter, r *http.Request) {
	apiKey := extractEventStreamId(r.URL.Path)
	if apiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "missing api key id", http.StatusBadRequest)
		return
	}
	apiKeyService := provider.NewEventStreamIdProvider().GetEventStreamIdService()
	if err := apiKeyService.RevokeEventStreamId(apiKey); err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "failed to revoke api key", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("api key revoked"))
}
