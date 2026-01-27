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
	"net/http"
	"strings"
	"sync"

	adminConfigService "github.com/wso2/identity-customer-data-service/internal/admin_config/service"
	"github.com/wso2/identity-customer-data-service/internal/system/security"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
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
func (psh *ProfileSchemaHandler) AddProfileSchemaAttributesForScope(w http.ResponseWriter, r *http.Request) {

	scope := r.PathValue("scope")
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	err := security.AuthnAndAuthz(r, "profile_schema:create")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	var schemaAttributes []model.ProfileSchemaAttribute
	if err := json.NewDecoder(r.Body).Decode(&schemaAttributes); err != nil {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Code,
			Message:     errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Message,
			Description: utils.HandleDecodeError(err, "profile schema attributes"),
		}, http.StatusBadRequest)
		utils.WriteErrorResponse(w, clientError)
		return
	}
	// Validate the scope
	if scope == constants.IdentityAttributes {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Code,
			Message:     errors2.PROFILE_SCHEMA_ADD_BAD_REQUEST.Message,
			Description: "Identity attributes cannot be created via this endpoint.",
		}, http.StatusBadRequest)
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
		schemaAttributes[i].OrgId = orgHandle
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	err = schemaService.AddProfileSchemaAttributesForScope(schemaAttributes, scope, orgHandle)
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

	orgHandle := utils.ExtractOrgHandleFromPath(r)
	err := security.AuthnAndAuthz(r, "profile_schema:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	profileSchema, err := schemaService.GetProfileSchema(orgHandle)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profileSchema)
}

// GetProfileSchemaAttributeById handles fetching the entire profile schema.
func (psh *ProfileSchemaHandler) GetProfileSchemaAttributeById(w http.ResponseWriter, r *http.Request) {

	scope := r.PathValue("scope")
	attributeId := r.PathValue("attrID")
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	err := security.AuthnAndAuthz(r, "profile_schema:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	attribute := model.ProfileSchemaAttribute{}
	if constants.AllowedAttributesScope[scope] {
		attribute, err = schemaService.GetProfileSchemaAttributeById(orgHandle, attributeId)
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
func (psh *ProfileSchemaHandler) GetProfileSchemaAttributeForScope(w http.ResponseWriter, r *http.Request) {

	scope := r.PathValue("scope")
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	err := security.AuthnAndAuthz(r, "profile_schema:view")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	var attributes interface{}

	// Build the filter from query params
	queryFilters := r.URL.Query()[constants.Filter] // Slice of filter params

	var filters []string
	for _, f := range queryFilters {
		// Split by " and " to support multiple conditions in a single filter param
		splitFilters := strings.Split(f, " and ")
		for _, sf := range splitFilters {
			sf = strings.TrimSpace(sf)
			if sf != "" {
				filters = append(filters, sf)
			}
		}
	}
	if len(queryFilters) > 0 {
		attributes, err = schemaService.GetProfileSchemaAttributesByScopeAndFilter(orgHandle, scope, filters)
	} else {
		if constants.AllowedAttributesScope[scope] {
			attributes, err = schemaService.GetProfileSchemaAttributesByScope(orgHandle, scope)
		}
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
func (psh *ProfileSchemaHandler) PatchProfileSchemaAttributeById(w http.ResponseWriter, r *http.Request) {

	attributeId := r.PathValue("attrID")
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	err := security.AuthnAndAuthz(r, "profile_schema:update")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		// todo: validate whats there in updates. Need to restrict certain fields from being updated
		// todo: name can
		return
	}
	err = schemaService.PatchProfileSchemaAttributeById(orgHandle, attributeId, updates)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	attribute, err := schemaService.GetProfileSchemaAttributeById(orgHandle, attributeId)
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

	orgHandle := utils.ExtractOrgHandleFromPath(r)
	err := security.AuthnAndAuthz(r, "profile_schema:delete")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()
	err = schemaService.DeleteProfileSchema(orgHandle)

	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteProfileSchemaAttributeById removes a profile schema attribute.
func (psh *ProfileSchemaHandler) DeleteProfileSchemaAttributeById(w http.ResponseWriter, r *http.Request) {

	attributeId := r.PathValue("attrID")
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	err := security.AuthnAndAuthz(r, "profile_schema:delete")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	schemaProvider := provider.NewProfileSchemaProvider()
	schemaService := schemaProvider.GetProfileSchemaService()

	err = schemaService.DeleteProfileSchemaAttributeById(orgHandle, attributeId)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func (psh *ProfileSchemaHandler) DeleteProfileSchemaAttributeForScope(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnAndAuthz(r, "profile_schema:delete")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	scope := r.PathValue("scope")
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
	orgHandle := utils.ExtractOrgHandleFromPath(r)

	err = schemaService.DeleteProfileSchemaAttributesByScope(orgHandle, scope)

	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

func (psh *ProfileSchemaHandler) SyncProfileSchema(w http.ResponseWriter, r *http.Request) {

	err := security.AuthnWithAdminCredentials(r)
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	var schemaSync model.ProfileSchemaSync
	if err := json.NewDecoder(r.Body).Decode(&schemaSync); err != nil {
		http.Error(w, "Invalid request.", http.StatusBadRequest)
		return
	}
	orgId := schemaSync.OrgId
	if orgId == "" {
		errMsg := fmt.Sprintf("Organization handle cannot be empty in sync event: %s", schemaSync.Event)
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_PROFILE.Code,
			Message:     errors2.UPDATE_PROFILE.Message,
			Description: errMsg,
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}
	logger := log.GetLogger()
	if !isCDSEnabled(orgId) {
		errMsg := "Unable to process profile sync event as CDS is not enabled for tenant: " + orgId
		logger.Info(errMsg)
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CDS_NOT_ENABLED.Code,
			Message:     errors2.CDS_NOT_ENABLED.Message,
			Description: errMsg,
		}, http.StatusBadRequest)
		utils.HandleError(w, clientError)
		return
	}

	logger.Info(fmt.Sprintf("Received schema sync request: %s for tenant: %s ", schemaSync.Event, schemaSync.OrgId))

	if schemaSync.Event == constants.AddScimAttributeEvent || schemaSync.Event == constants.UpdateScimAttributeEvent ||
		schemaSync.Event == constants.DeleteScimAttributeEvent || schemaSync.Event == constants.UpdateLocalAttributeEvent {
		// Enqueue the schema sync job for asynchronous processing
		if !workers.EnqueueSchemaSyncJob(schemaSync) {
			errMsg := fmt.Sprintf("Unable to process schema sync request for tenant: %s. The system is currently at capacity. Please try again in a few moments.", schemaSync.OrgId)
			logger.Error(errMsg)
			serverError := errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.SYNC_PROFILE_SCHEMA.Code,
				Message:     errors2.SYNC_PROFILE_SCHEMA.Message,
				Description: errMsg,
			}, fmt.Errorf("error enqueuing schema sync job"))
			utils.HandleError(w, serverError)
			return
		}

		logger.Debug("Schema sync request enqueued successfully for tenant: " + schemaSync.OrgId)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Profile schema sync request accepted for processing."})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unknown sync event."})

}

// isCDSEnabled checks if CDS is enabled for the given tenant
func isCDSEnabled(tenantId string) bool {
	return adminConfigService.GetAdminConfigService().IsCDSEnabled(tenantId)
}
