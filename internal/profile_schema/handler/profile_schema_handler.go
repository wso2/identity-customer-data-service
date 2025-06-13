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
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"net/http"
	"strings"
	"sync"
)

type ProfileSchemaHandler struct {
	store map[string]model.ProfileSchemaAttribute
	mu    *sync.RWMutex
}

func NewProfileSchemaHandler() *ProfileSchemaHandler {

	return &ProfileSchemaHandler{
		store: make(map[string]model.ProfileSchemaAttribute),
		mu:    &sync.RWMutex{},
	}
}

// AddProfileSchemaAttribute handles adding a new profile schema attribute.
func (psh *ProfileSchemaHandler) AddProfileSchemaAttribute(w http.ResponseWriter, r *http.Request) {

	var schemaAtt model.ProfileSchemaAttribute
	if err := json.NewDecoder(r.Body).Decode(&schemaAtt); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	if schemaAtt.AttributeId == "" {
		schemaAtt.AttributeId = uuid.NewString()
	}
	schemaAtt.OrgId = extractOrgIDFromPath(r)
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	err := schemaService.AddProfileSchemaAttribute(schemaAtt)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// GetProfileSchema handles fetching the entire profile schema.
func (psh *ProfileSchemaHandler) GetProfileSchema(w http.ResponseWriter, r *http.Request) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := extractOrgIDFromPath(r)
	profileSchema, err := schemaService.GetProfileSchema(orgId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileSchema)
}

// GetProfileSchemaAttribute handles fetching the entire profile schema.
func (psh *ProfileSchemaHandler) GetProfileSchemaAttribute(w http.ResponseWriter, r *http.Request) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := extractOrgIDFromPath(r)
	attributeId := extractAttributeId(r)
	attribute, err := schemaService.GetProfileSchemaAttribute(orgId, attributeId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(attribute)
}

// PatchProfileSchemaAttribute updates a profile schema attribute.
func (psh *ProfileSchemaHandler) PatchProfileSchemaAttribute(w http.ResponseWriter, r *http.Request) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := extractOrgIDFromPath(r)
	attributeId := extractAttributeId(r)

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	err := schemaService.PatchProfileSchemaAttribute(orgId, attributeId, updates)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	attribute, err := schemaService.GetProfileSchemaAttribute(orgId, attributeId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(attribute)
}

// DeleteProfileSchema removes the entire profile schema.
func (psh *ProfileSchemaHandler) DeleteProfileSchema(w http.ResponseWriter, r *http.Request) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := extractOrgIDFromPath(r)
	err := schemaService.DeleteProfileSchema(orgId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteProfileSchemaAttribute removes a profile schema attribute.
func (psh *ProfileSchemaHandler) DeleteProfileSchemaAttribute(w http.ResponseWriter, r *http.Request) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := extractOrgIDFromPath(r)
	attributeId := extractAttributeId(r)
	err := schemaService.DeleteProfileSchemaAttribute(orgId, attributeId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func extractOrgIDFromPath(r *http.Request) string {
	path := r.URL.Path
	parts := strings.Split(path, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "t" {
			return parts[i+1]
		}
	}
	return "carbon.super" // fallback default
}

func extractAttributeId(r *http.Request) string {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) > 0 {
		return pathParts[len(pathParts)-1]
	}
	return ""
}
