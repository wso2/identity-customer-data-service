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
	"fmt"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"net/http"
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

// AddProfileSchemaAttributesForScope handles adding a new profile schema attribute.
func (psh *ProfileSchemaHandler) AddProfileSchemaAttributesForScope(w http.ResponseWriter, r *http.Request, scope string) {

	var schemaAttributes []model.ProfileSchemaAttribute
	orgId := utils.ExtractTenantIdFromPath(r)
	if err := json.NewDecoder(r.Body).Decode(&schemaAttributes); err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Code,
			Message:     errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Message,
			Description: utils.HandleDecodeError(err, "profile schema attributes", "array"),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}
	// Validate the scope
	if scope == constants.IdentityAttributes {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Code,
			Message:     errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Message,
			Description: "Identity attributes cannot be created via this endpoint. Use the user management instead.",
		}, http.StatusBadRequest)
		//todo: decide on the status code
		utils.WriteErrorResponse(w, clientError)
		return
	}
	if !constants.AllowedAttributesScope[scope] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Code,
			Message:     errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Message,
			Description: "Invalid scope for profile schema attribute: " + scope,
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}
	if len(schemaAttributes) == 0 {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Code,
			Message:     errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Message,
			Description: "No attributes provided in the request body.",
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}
	// Generate a new UUID for each attribute if not provided
	for i := range schemaAttributes {
		if schemaAttributes[i].AttributeId == "" {
			schemaAttributes[i].AttributeId = uuid.New().String()
		}
		if schemaAttributes[i].Mutability == "" {
			schemaAttributes[i].Mutability = constants.MutabilityReadWrite
		}
		schemaAttributes[i].OrgId = orgId
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	err := schemaService.AddProfileSchemaAttributesForScope(schemaAttributes, scope)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	schemaAttributesNew := schemaAttributes
	for i := range schemaAttributesNew {
		schemaAttributesNew[i].OrgId = ""
		//todo: See if we need responseObject and covert to that
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(schemaAttributes)
}

// GetProfileSchema handles fetching the entire profile schema.
func (psh *ProfileSchemaHandler) GetProfileSchema(w http.ResponseWriter, r *http.Request) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := utils.ExtractTenantIdFromPath(r)
	profileSchema, err := schemaService.GetProfileSchema(orgId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileSchema)
}

// GetProfileSchemaAttributeById handles fetching the entire profile schema.
func (psh *ProfileSchemaHandler) GetProfileSchemaAttributeById(w http.ResponseWriter, r *http.Request, scope, attributeId string) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := utils.ExtractTenantIdFromPath(r)
	attribute := model.ProfileSchemaAttribute{}
	var err error
	if constants.AllowedAttributesScope[scope] {
		attribute, err = schemaService.GetProfileSchemaAttributeById(orgId, attributeId)
	}
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(attribute)
}

// GetProfileSchemaAttributeForScope handles fetching the entire profile schema for the scope.
func (psh *ProfileSchemaHandler) GetProfileSchemaAttributeForScope(w http.ResponseWriter, r *http.Request, scope string) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := utils.ExtractTenantIdFromPath(r)
	var attributes interface{}
	var err error
	if constants.AllowedAttributesScope[scope] {
		attributes, err = schemaService.GetProfileSchemaAttributesByScope(orgId, scope)
	}

	//todo: ensure it works for app data
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(attributes)
}

// PatchProfileSchemaAttributeById updates a profile schema attribute.
func (psh *ProfileSchemaHandler) PatchProfileSchemaAttributeById(w http.ResponseWriter, r *http.Request, scope, attributeId string) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := utils.ExtractTenantIdFromPath(r)
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	err := schemaService.PatchProfileSchemaAttributeById(orgId, attributeId, updates)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	attribute, err := schemaService.GetProfileSchemaAttributeById(orgId, attributeId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(attribute)
}

// PatchProfileSchemaAttributesForScope updates a profile schema attribute.
func (psh *ProfileSchemaHandler) PatchProfileSchemaAttributesForScope(w http.ResponseWriter, r *http.Request, scope string) {

	if scope == constants.IdentityAttributes {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: "Identity attributes cannot be modified via this endpoint.",
		}, http.StatusMethodNotAllowed)
		utils.WriteErrorResponse(w, clientError)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := utils.ExtractTenantIdFromPath(r)
	var updates []map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	err := schemaService.PatchProfileSchemaAttributesByScope(orgId, scope, updates)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	attribute, err := schemaService.GetProfileSchemaAttributesByScope(orgId, scope)
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
	orgId := utils.ExtractTenantIdFromPath(r)
	err := schemaService.DeleteProfileSchema(orgId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteProfileSchemaAttributeById removes a profile schema attribute.
func (psh *ProfileSchemaHandler) DeleteProfileSchemaAttributeById(w http.ResponseWriter, r *http.Request, scope, attributeId string) {

	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := utils.ExtractTenantIdFromPath(r)

	err := schemaService.DeleteProfileSchemaAttributeById(orgId, attributeId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func (psh *ProfileSchemaHandler) DeleteProfileSchemaAttributeForScope(w http.ResponseWriter, r *http.Request, scope string) {

	if scope == constants.IdentityAttributes {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: "Identity attributes cannot be created or modified via this endpoint.",
		}, http.StatusMethodNotAllowed)
		utils.WriteErrorResponse(w, clientError)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	orgId := utils.ExtractTenantIdFromPath(r)

	err := schemaService.DeleteProfileSchemaAttributesByScope(orgId, scope)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func (psh *ProfileSchemaHandler) SyncProfileSchema(w http.ResponseWriter, r *http.Request) {

	var schemaAtt model.ProfileSchemaSync
	if err := json.NewDecoder(r.Body).Decode(&schemaAtt); err != nil {
		http.Error(w, "Invalid request.", http.StatusBadRequest)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()

	log.GetLogger().Info(fmt.Sprintf("Received schema sync request: %s for tenant: %s ", schemaAtt.Event, schemaAtt.OrgId))

	if schemaAtt.Event == constants.SchemaInitEvent {
		orgId := schemaAtt.OrgId
		err := schemaService.SyncProfileSchema(orgId)
		if err != nil {
			utils.HandleError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Profile schema synced successfully"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unknown sync event."})

}
