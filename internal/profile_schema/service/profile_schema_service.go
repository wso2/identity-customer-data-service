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

package service

import (
	"encoding/json"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	psstr "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"github.com/wso2/identity-customer-data-service/internal/system/client"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strings"
)

type ProfileSchemaServiceInterface interface {
	GetProfileSchema(orgId string) (map[string]interface{}, error)
	DeleteProfileSchema(orgId string) error
	AddProfileSchemaAttributesForScope(attrs []model.ProfileSchemaAttribute, scope string) error
	GetProfileSchemaAttributesByScope(orgId, scope string) (interface{}, error)
	GetProfileSchemaAttributesByScopeAndFilter(id, scope string, filters []string) (interface{}, error)
	DeleteProfileSchemaAttributesByScope(orgId, scope string) error
	GetProfileSchemaAttributeById(orgId, attributeId string) (model.ProfileSchemaAttribute, error)
	PatchProfileSchemaAttributeById(orgId, attributeId string, updates map[string]interface{}) error
	DeleteProfileSchemaAttributeById(orgId, attributeId string) error
	SyncProfileSchema(orgId string) error
}

// ProfileSchemaService is the default implementation of the ProfileSchemaServiceInterface.
type ProfileSchemaService struct{}

// GetProfileSchemaService creates a new instance of UnificationRuleService.
func GetProfileSchemaService() ProfileSchemaServiceInterface {

	return &ProfileSchemaService{}
}

// AddProfileSchemaAttributesForScope adds profile schema attributes to the specific scope.
func (s *ProfileSchemaService) AddProfileSchemaAttributesForScope(schemaAttributes []model.ProfileSchemaAttribute, scope string) error {

	validAttrs := make([]model.ProfileSchemaAttribute, 0, len(schemaAttributes))
	for _, attr := range schemaAttributes {
		if err, isValid := s.validateSchemaAttribute(attr); isValid {
			validAttrs = append(validAttrs, attr)
		} else {
			return err
		}

		// Ensure the scope is valid
		parts := strings.SplitN(attr.AttributeName, ".", 2)
		scopeOfAttr := parts[0]
		if scope != scopeOfAttr {
			errorMsg := fmt.Sprintf("Attribute '%s' does not match the api scope '%s'", attr.AttributeName, scope)
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
				Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
				Description: errorMsg,
			}, http.StatusBadRequest)
			return clientError
		}

		existing, err := psstr.GetProfileSchemaAttributeByName(attr.OrgId, attr.AttributeName)
		if err != nil {
			return err
		}
		if existing != nil {
			errorMsg := fmt.Sprintf("Attribute '%s' already exists for org '%s'", attr.AttributeName, attr.OrgId)
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.SCHEMA_ATTRIBUTE_ALREADY_EXISTS.Code,
				Message:     errors2.SCHEMA_ATTRIBUTE_ALREADY_EXISTS.Message,
				Description: errorMsg,
			}, http.StatusConflict)
			return clientError
		}
	}

	return psstr.AddProfileSchemaAttributesForScope(validAttrs, scope)
}

func (s *ProfileSchemaService) validateSchemaAttribute(attr model.ProfileSchemaAttribute) (error, bool) {

	parts := strings.Split(attr.AttributeName, ".")
	if len(parts) < 2 {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: "Attribute name must be in the format <Scope>.<Name>",
		}, http.StatusBadRequest)
		return clientError, false
	}

	scope := parts[0]
	if !constants.AllowedAttributesScope[scope] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:    errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message: errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: fmt.Sprintf("Invalid scope: %s. Must be one of identity_attributes, traits, "+
				"application_data", scope),
		}, http.StatusBadRequest)
		return clientError, false
	}

	if scope == constants.ApplicationData {
		if attr.ApplicationIdentifier == "" {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
				Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
				Description: "Application identifier is required for application_data scope",
			}, http.StatusBadRequest)
			return clientError, false
		}
	}

	if !constants.AllowedValueTypes[attr.ValueType] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: fmt.Sprintf("Invalid value_type: %s. Must be one of %v", attr.ValueType, keysOf(constants.AllowedValueTypes)),
		}, http.StatusBadRequest)
		return clientError, false
	}

	if len(attr.SubAttributes) > 0 {
		if attr.ValueType != constants.ComplexDataType {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
				Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
				Description: fmt.Sprintf("SubAttributes are meant for for value_type: %s", constants.ComplexDataType),
			}, http.StatusBadRequest)
			return clientError, false
		}
	}
	if attr.ValueType == constants.ComplexDataType {
		if attr.SubAttributes == nil || len(attr.SubAttributes) == 0 {
			clientError := errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
				Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
				Description: fmt.Sprintf("SubAttributes are required for value_type: %s", constants.ComplexDataType),
			}, http.StatusBadRequest)
			return clientError, false
		}

		for _, subAttr := range attr.SubAttributes {
			if !strings.HasPrefix(subAttr.AttributeName, attr.AttributeName+".") {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
					Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
					Description: fmt.Sprintf("Invalid sub-attribute name: %s. It must start with parent attribute name '%s' followed by a dot and sub-key.", subAttr.AttributeName, attr.AttributeName),
				}, http.StatusBadRequest)
				return clientError, false
			}

			subAttribute, err := s.GetProfileSchemaAttributeById(attr.OrgId, subAttr.AttributeId)
			if err != nil {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
					Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
					Description: fmt.Sprintf("Sub-attribute with Id '%s' does not exist.", subAttr.AttributeId),
				}, http.StatusBadRequest)
				return clientError, false
			}
			if subAttribute.AttributeName != subAttr.AttributeName {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
					Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
					Description: fmt.Sprintf("Sub-attribute name '%s' does not match the expected name.", subAttr.AttributeName),
				}, http.StatusBadRequest)
				return clientError, false
			}
			if subAttribute.ApplicationIdentifier != attr.ApplicationIdentifier {
				clientError := errors2.NewClientError(errors2.ErrorMessage{
					Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
					Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
					Description: fmt.Sprintf("Sub-attribute '%s' must have the same application identifier as the parent attribute '%s'.", subAttr.AttributeName, attr.AttributeName),
				}, http.StatusBadRequest)
				return clientError, false
			}
		}
	}

	if !constants.AllowedMutabilityValues[attr.Mutability] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: fmt.Sprintf("Invalid mutability: %s. Must be one of %v", attr.ValueType, keysOf(constants.AllowedMutabilityValues)),
		}, http.StatusBadRequest)
		return clientError, false
	}

	if !constants.AllowedMergeStrategies[attr.MergeStrategy] {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: fmt.Sprintf("Invalid merge strategy: %s. Must be one of %v", attr.MergeStrategy, keysOf(constants.AllowedMergeStrategies)),
		}, http.StatusBadRequest)
		return clientError, false
	}
	return nil, true
}

func (s *ProfileSchemaService) GetProfileSchemaAttributeById(orgId, attributeId string) (model.ProfileSchemaAttribute, error) {
	return psstr.GetProfileSchemaAttributeById(orgId, attributeId)
}

// GetProfileSchemaAttributesByScope retrieves profile schema attributes for a specific scope.
func (s *ProfileSchemaService) GetProfileSchemaAttributesByScope(orgId, scope string) (interface{}, error) {

	schemaAttributes, err := psstr.GetProfileSchemaAttributesByScope(orgId, scope)
	if err != nil {
		return nil, err
	}

	// todo: ensure to have the addition also in same format
	if scope == constants.ApplicationData {
		grouped := make(map[string][]model.ProfileSchemaAttribute)
		for _, attr := range schemaAttributes {
			appID := attr.ApplicationIdentifier
			if appID == "" {
				log.GetLogger().Warn(fmt.Sprintf("Missing application identifier for application data: %s", attr.AttributeName))
				continue
			}
			// Keep application_identifier in the attribute as required
			grouped[appID] = append(grouped[appID], attr)
		}
		return grouped, err
	}
	return schemaAttributes, nil
}

func (s *ProfileSchemaService) PatchProfileSchemaAttributeById(orgId, attributeId string, updates map[string]interface{}) error {

	if len(updates) == 0 {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: "No updates provided for the profile schema attribute",
		}, http.StatusBadRequest)
	}
	attribute, err := s.GetProfileSchemaAttributeById(orgId, attributeId)
	if err != nil {
		return err
	}
	if attribute.AttributeId == "" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: fmt.Sprintf("Attribute with Id '%s' does not exist", attributeId),
		}, http.StatusNotFound)
	}

	// attribute id cannot be there and also org id. attribute name only can be updated not the scope.
	var canonicalValues []model.CanonicalValue
	if cv, ok := updates["canonical_values"]; ok && cv != nil {
		if cvSlice, ok := cv.([]model.CanonicalValue); ok {
			canonicalValues = cvSlice
		}
	} else {
		canonicalValues = attribute.CanonicalValues // Keep existing canonical values if not updated
	}

	var subAttributes []model.SubAttribute
	if saRaw, ok := updates["sub_attributes"]; ok && saRaw != nil {
		saSlice, ok := saRaw.([]interface{})
		if ok {
			for _, item := range saSlice {
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				// Convert map to JSON, then to SubAttribute
				itemBytes, err := json.Marshal(itemMap)
				if err != nil {
					continue
				}

				var subAttr model.SubAttribute
				if err := json.Unmarshal(itemBytes, &subAttr); err == nil {
					subAttributes = append(subAttributes, subAttr)
				}
			}
		}
	} else {
		subAttributes = attribute.SubAttributes // fallback to existing
	}

	var applicationIdentifier string
	if appID, ok := updates["application_identifier"]; ok && appID != nil {
		if appIDStr, ok := appID.(string); ok && appIDStr != "" {
			applicationIdentifier = appIDStr
		} else {
			// If application_identifier is not provided or is empty, keep the existing one
			applicationIdentifier = attribute.ApplicationIdentifier
		}
	} else {
		applicationIdentifier = attribute.ApplicationIdentifier // Keep existing application identifier if not updated
	}

	multiValued := false // Default to false if not provided
	if mv, ok := updates["multi_valued"]; ok && mv != nil {
		if mvBool, ok := mv.(bool); ok {
			multiValued = mvBool
		} else {
			return errors2.NewClientError(errors2.ErrorMessage{
				Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
				Message:     "Invalid value for multi_valued",
				Description: "multi_valued must be a boolean",
			}, http.StatusBadRequest)
		}
	} else {
		log.GetLogger().Debug(fmt.Sprintf("multi_valued not provided in patch; defaulting to false for attribute: %s", attributeId))
	}

	// todo: ensure NPE - see if u need to update application identifier also...its PUT as well. So yeah.
	err, isValid := s.validateSchemaAttribute(model.ProfileSchemaAttribute{
		OrgId:                 orgId,
		AttributeId:           attributeId,
		AttributeName:         updates["attribute_name"].(string),
		ValueType:             updates["value_type"].(string),
		MergeStrategy:         updates["merge_strategy"].(string),
		Mutability:            updates["mutability"].(string),
		MultiValued:           multiValued,
		CanonicalValues:       canonicalValues,
		SubAttributes:         subAttributes,
		ApplicationIdentifier: applicationIdentifier,
	})
	if !isValid {
		if err != nil {
			return err
		}
		// If validation fails, return a bad request error
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INVALID_ATTRIBUTE_NAME.Code,
			Message:     errors2.INVALID_ATTRIBUTE_NAME.Message,
			Description: "Invalid updates provided for the profile schema attribute",
		}, http.StatusBadRequest)
	}
	return psstr.PatchProfileSchemaAttributeById(orgId, attributeId, updates)
}

// DeleteProfileSchemaAttributeById deletes a profile schema attribute by its Id.
func (s *ProfileSchemaService) DeleteProfileSchemaAttributeById(orgId, attributeId string) error {

	//// Validate the attributeId format
	//parts := strings.Split(attributeId, ".")
	//scope := parts[0]
	//if scope == constants.IdentityAttributes {
	//	clientError := errors2.NewClientError(errors2.ErrorMessage{
	//		Code:        errors2.INVALID_OPERATION.Code,
	//		Message:     errors2.INVALID_OPERATION.Message,
	//		Description: "Identity attributes cannot be created or modified via this endpoint. Use the user management instead.",
	//	}, http.StatusMethodNotAllowed)
	//	return clientError
	//}
	return psstr.DeleteProfileSchemaAttributeById(orgId, attributeId)
}

func (s *ProfileSchemaService) DeleteProfileSchemaAttributesByScope(orgId, scope string) error {
	return psstr.DeleteProfileSchemaAttributes(orgId, scope)
}

// GetProfileSchema retrieves the complete profile schema for the given organization Id.
func (s *ProfileSchemaService) GetProfileSchema(orgId string) (map[string]interface{}, error) {

	logger := log.GetLogger()
	// Step 1: Flatten core schema fields from model.CoreSchema
	profileSchema := make(map[string]interface{})
	meta := map[string]map[string]string{}
	for attrName, core := range model.CoreSchema {
		if strings.HasPrefix(attrName, "meta.") {
			field := strings.TrimPrefix(attrName, "meta.")
			meta[field] = map[string]string{
				constants.ValueType:  core[constants.ValueType],
				constants.Mutability: core[constants.Mutability],
			}
		} else {
			profileSchema[attrName] = map[string]string{
				constants.ValueType:  core[constants.ValueType],
				constants.Mutability: core[constants.Mutability],
			}
		}
	}

	// Add meta to the profileSchema
	profileSchema["meta"] = meta

	// Step 2: Fetch schema attributes from DB
	schemaAttributes, err := psstr.GetProfileSchemaAttributesForOrg(orgId)
	if err != nil {
		errMsg := fmt.Sprintf("Error retrieving profile schema attributes for org %s: %v", orgId)
		logger.Debug(errMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_PROFILE_SCHEMA.Code,
			Message:     errors2.GET_PROFILE_SCHEMA.Message,
			Description: errMsg,
		}, err)
	}

	// Step 3: Group them by scope
	identityAttrs := make([]model.ProfileSchemaAttribute, 0)
	appDataAttrs := make(map[string][]model.ProfileSchemaAttribute)
	traitsAttrs := make([]model.ProfileSchemaAttribute, 0)

	for _, attr := range schemaAttributes {
		parts := strings.SplitN(attr.AttributeName, ".", 2)
		if len(parts) != 2 {
			logger.Warn(fmt.Sprintf("Invalid attribute name format: %s", attr.AttributeName))
			continue
		}
		scope := parts[0]

		switch scope {
		case constants.IdentityAttributes:
			identityAttrs = append(identityAttrs, attr)
		case constants.ApplicationData:
			if attr.ApplicationIdentifier == "" {
				logger.Warn(fmt.Sprintf("Missing application identifier for application data: %s", attr.AttributeName))
				continue
			}
			appDataAttrs[attr.ApplicationIdentifier] = append(appDataAttrs[attr.ApplicationIdentifier], attr)
		case constants.Traits:
			traitsAttrs = append(traitsAttrs, attr)
		default:
			logger.Warn(fmt.Sprintf("Unknown attribute scope: %s", scope))
		}
	}

	// Step 4: Add scoped arrays
	profileSchema[constants.IdentityAttributes] = identityAttrs
	profileSchema[constants.ApplicationData] = appDataAttrs
	profileSchema[constants.Traits] = traitsAttrs

	return profileSchema, nil
}

func (s *ProfileSchemaService) DeleteProfileSchema(orgId string) error {
	return psstr.DeleteProfileSchema(orgId)
}

func keysOf(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func GetProfileSchemaAttributesWithFilter(orgId string, filters []string) ([]model.ProfileSchemaAttribute, error) {

	allAttrs, err := psstr.GetProfileSchemaAttributesForOrg(orgId) // assuming this exists
	if err != nil {
		return nil, err
	}

	filtered := make([]model.ProfileSchemaAttribute, 0)

	for _, attr := range allAttrs {
		match := true

		for _, f := range filters {
			field, op, val, err := parseFilter(f)
			if err != nil {
				return nil, fmt.Errorf("invalid filter '%s': %v", f, err)
			}

			if !matches(attr, field, op, val) {
				match = false
				break
			}
		}

		if match {
			filtered = append(filtered, attr)
		}
	}

	return filtered, nil
}

func parseFilter(f string) (field, op, value string, err error) {
	parts := strings.SplitN(f, " ", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("must be in format: field op value")
	}
	return parts[0], parts[1], parts[2], nil
}

func matches(attr model.ProfileSchemaAttribute, field, op, val string) bool {
	switch field {
	case "attribute_name":
		switch op {
		case "eq":
			return attr.AttributeName == val
		case "contains":
			return strings.Contains(attr.AttributeName, val)
		}
	case "application_identifier":
		switch op {
		case "eq":
			return attr.ApplicationIdentifier == val
		case "contains":
			return strings.Contains(attr.ApplicationIdentifier, val)
		}
	}
	return false
}

func (s *ProfileSchemaService) SyncProfileSchema(orgId string) error {

	cfg := config.GetCDSRuntime().Config
	identityClient := client.NewIdentityClient(cfg)

	claims, err := identityClient.GetProfileSchema(orgId)
	logger := log.GetLogger()
	if err != nil {
		errMsg := fmt.Sprintf("failed to fetch profile schema from identity server for organization %s:", orgId)
		logger.Debug(errMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.SYNC_PROFILE_SCHEMA.Code,
			Message:     errors2.SYNC_PROFILE_SCHEMA.Message,
			Description: errMsg,
		}, err)
	}

	if len(claims) > 0 {
		err := psstr.UpsertIdentityAttributes(orgId, claims)
		if err != nil {
			errMsg := fmt.Sprintf("failed to persist profile schema for organization %s:", orgId)
			logger.Debug(errMsg, log.Error(err))
			return errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.SYNC_PROFILE_SCHEMA.Code,
				Message:     errors2.SYNC_PROFILE_SCHEMA.Message,
				Description: errMsg,
			}, err)
		}
		logger.Info("Profile schema successfully updated for org: " + orgId)
	}
	return nil
}

func (s *ProfileSchemaService) GetProfileSchemaAttributesByScopeAndFilter(orgId, scope string, filters []string) (interface{}, error) {

	schemaAttributes, err := psstr.GetProfileSchemaAttributesByScopeAndFilter(orgId, scope, filters)
	if err != nil {
		return nil, err
	}

	if scope == constants.ApplicationData {
		grouped := make(map[string][]model.ProfileSchemaAttribute)
		for _, attr := range schemaAttributes {
			appID := attr.ApplicationIdentifier
			if appID == "" {
				log.GetLogger().Warn(fmt.Sprintf("Missing application identifier for application data: %s", attr.AttributeName))
				continue
			}
			grouped[appID] = append(grouped[appID], attr)
		}
		return grouped, nil
	}
	return schemaAttributes, nil
}
