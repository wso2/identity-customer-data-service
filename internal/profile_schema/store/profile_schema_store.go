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

package store

import (
	"encoding/json"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strconv"
	"strings"
)

// AddProfileSchemaAttributesForScope adds multiple profile schema attributes.
func AddProfileSchemaAttributesForScope(attrs []model.ProfileSchemaAttribute, scope string) error {

	logger := log.GetLogger()
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while initializing DB client for adding profile schema attributes")
		logger.Debug(errorMsg, log.Error(err))
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	if len(attrs) == 0 {
		return nil // nothing to insert
	}

	baseQuery := scripts.InsertProfileSchemaAttributesForScope[provider.NewDBProvider().GetDBType()]
	valueStrings := make([]string, 0, len(attrs))
	valueArgs := make([]interface{}, 0, len(attrs)*11)

	for i, attr := range attrs {
		idx := i * 11
		subAttrsJSON, err := json.Marshal(attr.SubAttributes)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to marshal sub attributes for attribute %s", attr.AttributeId)
			logger.Debug(errorMsg, log.Error(err))
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.MARSHAL_JSON.Code,
				Message:     errors.MARSHAL_JSON.Message,
				Description: errorMsg,
			}, err)
		}
		canonicalJSON, err := json.Marshal(attr.CanonicalValues)

		if err != nil {
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.MARSHAL_JSON.Code,
				Message:     errors.MARSHAL_JSON.Message,
				Description: fmt.Sprintf("Failed to marshal canonical values for attribute %s", attr.AttributeId),
			}, err)
		}

		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d,  $%d, $%d,  $%d, $%d, $%d) ",
			idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8, idx+9, idx+10, idx+11))
		valueArgs = append(valueArgs, attr.OrgId, attr.AttributeId, attr.AttributeName, attr.ValueType,
			attr.MergeStrategy, attr.ApplicationIdentifier, attr.Mutability, attr.MultiValued, subAttrsJSON,
			canonicalJSON, scope)

	}

	query := baseQuery + strings.Join(valueStrings, ", ")
	_, err = dbClient.ExecuteQuery(query, valueArgs...)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to insert profile schema attributes for org: %s", attrs[0].OrgId)
		logger.Debug(errorMsg, log.Error(err))
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.ADD_PROFILE_SCHEMA.Code,
			Message:     errors.ADD_PROFILE_SCHEMA.Message,
			Description: errorMsg,
		}, err)
	}
	logger.Info(fmt.Sprintf("Successfully added %d profile schema attributes for organization: %s",
		len(attrs), attrs[0].OrgId))
	return nil
}

// GetProfileSchemaAttributeById retrieves a profile schema attribute by its ID for a given organization.
func GetProfileSchemaAttributeById(orgId, attributeId string) (model.ProfileSchemaAttribute, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while fetching profile schema for org: %s and attribute: %s",
			orgId, attributeId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return model.ProfileSchemaAttribute{}, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfileSchemaAttributeById[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, orgId, attributeId)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while fetching profile schema for the org:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.GET_PROFILE_SCHEMA.Code,
			Message:     errors.GET_PROFILE_SCHEMA.Message,
			Description: errorMsg,
		}, err)
		return model.ProfileSchemaAttribute{}, serverError
	}
	if len(results) == 0 {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ATTRIBUTE_NOT_FOUND.Code,
			Message:     errors.ATTRIBUTE_NOT_FOUND.Message,
			Description: "Profile schema attribute not found for org: " + orgId + " and attribute: " + attributeId,
		}, http.StatusNotFound)
		return model.ProfileSchemaAttribute{}, clientError
	}
	row := results[0]
	return mapRowToProfileAttribute(row), nil
}

// GetProfileSchemaAttributesByScope retrieves all profile schema attributes for a given organization and scope.
func GetProfileSchemaAttributesByScope(orgId, scope string) ([]model.ProfileSchemaAttribute, error) {

	logger := log.GetLogger()
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		errorMsg := fmt.Sprintf("Error initializing DB client for org: %s and scope: %s", orgId, scope)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	query := scripts.GetProfileSchemaAttributeByScope[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, orgId, scope)
	if err != nil {
		errorMsg := fmt.Sprintf("Error fetching profile schema attributes for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors.NewServerError(errors.ErrorMessage{
			Code:        errors.GET_PROFILE_SCHEMA.Code,
			Message:     errors.GET_PROFILE_SCHEMA.Message,
			Description: errorMsg,
		}, err)
	}

	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No profile schema attributes found for org: %s and scope: %s", orgId, scope))
		return []model.ProfileSchemaAttribute{}, nil
	}

	attributes := make([]model.ProfileSchemaAttribute, 0, len(results))
	for _, row := range results {
		attributes = append(attributes, mapRowToProfileAttribute(row))
	}

	return attributes, nil
}

// GetProfileSchemaAttributeByName retrieves a profile schema attribute by its name for a given organization.
func GetProfileSchemaAttributeByName(orgId, attributeName string) (*model.ProfileSchemaAttribute, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()

	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get database client for fetching schema attribute: %s", attributeName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfileSchemaAttributeByName[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, orgId, attributeName)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed in fetching schema attribute '%s' for organization '%s'", attributeName,
			orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("No profile schema found for attribute: %s in org: %s", attributeName, orgId))
		return nil, nil
	}

	row := results[0]
	var subAttrs []model.SubAttribute
	if raw, ok := row["sub_attributes"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &subAttrs); err != nil {
			errorMsg := fmt.Sprintf("Failed to unmarshal sub_attributes for attribute '%s' in org '%s'",
				attributeName, orgId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors.NewServerError(errors.ErrorMessage{
				Code:        errors.UNMARSHAL_JSON.Code,
				Message:     errors.UNMARSHAL_JSON.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
	}

	var canonicalValues []model.CanonicalValue
	if raw, ok := row["canonical_values"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &canonicalValues); err != nil {
			errorMsg := fmt.Sprintf("Failed to unmarshal canonical_values for attribute '%s' in organization '%s'",
				attributeName, orgId)
			logger.Debug(errorMsg, log.Error(err))
			serverError := errors.NewServerError(errors.ErrorMessage{
				Code:        errors.UNMARSHAL_JSON.Code,
				Message:     errors.UNMARSHAL_JSON.Message,
				Description: errorMsg,
			}, err)
			return nil, serverError
		}
	}

	attr := &model.ProfileSchemaAttribute{
		OrgId:                 orgId,
		AttributeName:         row["attribute_name"].(string),
		ValueType:             row["value_type"].(string),
		MergeStrategy:         row["merge_strategy"].(string),
		Mutability:            row["mutability"].(string),
		ApplicationIdentifier: row["application_identifier"].(string),
		MultiValued:           row["multi_valued"].(bool),
		SubAttributes:         subAttrs,
		CanonicalValues:       canonicalValues,
	}

	logger.Info(fmt.Sprintf("Successfully fetched profile schema attribute '%s' for organizaton '%s'",
		attributeName, orgId))
	return attr, nil
}

// GetProfileSchemaAttributesForOrg retrieves all profile schema attributes for a given organization.
func GetProfileSchemaAttributesForOrg(orgId string) ([]model.ProfileSchemaAttribute, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while fetching profile schema for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfileSchemaByOrg[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, orgId)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while fetching profile schema for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return nil, serverError
	}

	var schema []model.ProfileSchemaAttribute
	for _, row := range results {
		attr := mapRowToProfileAttribute(row)
		schema = append(schema, attr)
	}
	return schema, nil
}

// PatchProfileSchemaAttributeById updates a specific profile schema attribute for a given organization.
func PatchProfileSchemaAttributeById(orgId, attributeId string, updates map[string]interface{}) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while patching profile schema for org: %s and attribute: %s", orgId, attributeId)
		logger.Debug(errorMsg, log.Error(err))
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	for key, value := range updates {
		switch v := value.(type) {
		case []interface{}, map[string]interface{}:
			// Marshal slices/maps to JSON
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("failed to marshal %s to JSON: %w", key, err)
			}
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
			args = append(args, string(jsonBytes))

		case nil:
			// Skip nil values to avoid null pointer issues
			continue

		default:
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
			args = append(args, v)
		}
		argIndex++
	}

	// Append WHERE clause parameters
	args = append(args, orgId, attributeId)

	query := `UPDATE profile_schema SET ` + strings.Join(setClauses, ", ") +
		` WHERE tenant_id = $` + strconv.Itoa(argIndex) + ` AND attribute_id = $` + strconv.Itoa(argIndex+1)

	_, err = dbClient.ExecuteQuery(query, args...)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while executing update for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
	}

	return nil
}

// DeleteProfileSchemaAttributeById deletes a specific profile schema attribute by its ID for a given organization.
func DeleteProfileSchemaAttributeById(orgId, attributeId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while deleting profile schema for org: %s and attribute: %s", orgId, attributeId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.DeleteProfileSchemaAttributeById[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, orgId, attributeId)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while deleting profile schema attribute with id: %s", attributeId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info(fmt.Sprintf("Profile schema attribute: %s deleted: ", attributeId))
	return nil
}

// DeleteProfileSchemaAttributes deletes all profile schema attributes for a given organization and scope.
func DeleteProfileSchemaAttributes(orgId, scope string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while deleting profile schema for org: %s and attribute: %s", orgId, scope)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.DeleteProfileSchemaAttributeForScope[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, orgId, scope)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while deleting profile schema attribute with scope: %s", scope)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info(fmt.Sprintf("Profile schema attributes with scope:%s deleted for the organization:%s ", scope, orgId))
	return nil
}

func PatchProfileSchemaAttributesForScope(orgId string, scope string, updates []model.ProfileSchemaAttribute) error {

	logger := log.GetLogger()
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		errorMsg := fmt.Sprintf("Error initializing DB client for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for update of profile schema attributes for organization: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_BEGN_TRANSACTION.Code,
			Message:     errors.DB_BEGN_TRANSACTION.Message,
			Description: errorMsg,
		}, err)
	}

	stmt := scripts.UpdateProfileSchemaAttributesForSchema[provider.NewDBProvider().GetDBType()]

	for _, attr := range updates {
		subAttrsJSON, err := json.Marshal(attr.SubAttributes)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to marshal sub attributes for attribute %s", attr.AttributeId)
			logger.Debug(errorMsg, log.Error(err))
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.MARSHAL_JSON.Code,
				Message:     errors.MARSHAL_JSON.Message,
				Description: errorMsg,
			}, err)
		}
		canonicalJSON, err := json.Marshal(attr.CanonicalValues)

		if err != nil {
			errorMsg := fmt.Sprintf("Failed to marshal canonical values for attribute %s", attr.AttributeId)
			logger.Debug(errorMsg, log.Error(err))
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.MARSHAL_JSON.Code,
				Message:     errors.MARSHAL_JSON.Message,
				Description: errorMsg,
			}, err)
		}
		args := []interface{}{
			attr.AttributeName,
			attr.ValueType,
			attr.MergeStrategy,
			attr.Mutability,
			attr.ApplicationIdentifier,
			attr.MultiValued,
			canonicalJSON,
			subAttrsJSON,
			orgId,
			attr.AttributeId,
			scope,
		}

		if _, err := tx.Exec(stmt, args...); err != nil {
			tx.Rollback()
			errorMsg := fmt.Sprintf("Failed to update attribute %s for organization %s", attr.AttributeId, orgId)
			logger.Debug(errorMsg, log.Error(err))
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.EXECUTE_QUERY.Code,
				Message:     errors.EXECUTE_QUERY.Message,
				Description: errorMsg,
			}, err)
		}
	}

	if err := tx.Commit(); err != nil {
		errorMsg := fmt.Sprintf("Failed to commit transaction for updating profile schema attributes for organization: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_COMMIT_TRANSACTION.Code,
			Message:     errors.DB_COMMIT_TRANSACTION.Message,
			Description: errorMsg,
		}, err)
	}

	logger.Info(fmt.Sprintf("Update completed for %d profile schema attributes of scope: %s of organization: %s )", len(updates), scope, orgId))
	return nil
}

// DeleteProfileSchema deletes all profile schema attributes for a given organization.
func DeleteProfileSchema(orgId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while deleting all of profile schema attributes for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := scripts.DeleteProfileSchemaForOrg[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, orgId)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while deleting all of profile schema attributes for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	logger.Info("Profile schema cleared for org: " + orgId)
	return nil
}

// mapRowToProfileAttribute converts database row to a ProfileSchemaAttribute model.
func mapRowToProfileAttribute(row map[string]interface{}) model.ProfileSchemaAttribute {

	var subAttrs []model.SubAttribute
	if raw, ok := row["sub_attributes"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &subAttrs); err != nil {
			log.GetLogger().Debug("Failed to unmarshal sub_attributes", log.Error(err))
		}
	}

	var canonicalValues []model.CanonicalValue
	if raw, ok := row["canonical_values"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &canonicalValues); err != nil {
			log.GetLogger().Debug("Failed to unmarshal canonical_values", log.Error(err))
		}
	}

	return model.ProfileSchemaAttribute{
		AttributeId:           fmt.Sprint(row["attribute_id"]),
		AttributeName:         row["attribute_name"].(string),
		ValueType:             row["value_type"].(string),
		MergeStrategy:         row["merge_strategy"].(string),
		Mutability:            row["mutability"].(string),
		ApplicationIdentifier: row["application_identifier"].(string),
		MultiValued:           row["multi_valued"].(bool),
		SubAttributes:         subAttrs,
		CanonicalValues:       canonicalValues,
	}
}

func UpsertIdentityAttributes(orgID string, attrs []model.ProfileSchemaAttribute) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error initializing DB client for organization: %s ", orgID)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	// Step 1: Delete existing identity_attributes for this org
	deleteQuery := scripts.DeleteIdentityClaimsOfProfileSchema[provider.NewDBProvider().GetDBType()]
	if _, err := dbClient.ExecuteQuery(deleteQuery, orgID); err != nil {
		errorMsg := fmt.Sprintf("Failed to delete existing identity attributes of profile schema for organization: %s", orgID)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.SYNC_PROFILE_SCHEMA.Code,
			Message:     errors.SYNC_PROFILE_SCHEMA.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	// Step 2: Insert new attributes
	insertQuery := scripts.InsertIdentityClaimsForProfileSchema[provider.NewDBProvider().GetDBType()]

	var valueStrings []string
	var valueArgs []interface{}
	argIndex := 1

	for _, attr := range attrs {
		canonicalJSON, _ := json.Marshal(attr.CanonicalValues)
		subAttrJSON, _ := json.Marshal(attr.SubAttributes)
		attrKey := extractClaimKeyFromURI(attr.AttributeName)
		attr.AttributeName = attrKey

		valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d, $%d,$%d)",
			argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5, argIndex+6,
			argIndex+7, argIndex+8, argIndex+9, argIndex+10, argIndex+11, argIndex+12))
		valueArgs = append(valueArgs,
			orgID,
			attr.AttributeId,
			attr.AttributeName,
			attr.ValueType,
			attr.MergeStrategy,
			attr.Mutability,
			attr.ApplicationIdentifier,
			attr.MultiValued,
			string(canonicalJSON),
			string(subAttrJSON),
			attr.SCIMDialect,
			attr.MappedLocalClaim,
			constants.IdentityAttributes,
		)
		argIndex += 13
	}

	insertQuery += strings.Join(valueStrings, ",")
	if _, err := dbClient.ExecuteQuery(insertQuery, valueArgs...); err != nil {
		errorMsg := fmt.Sprintf("Failed to insert new identity attributes of profile schema for organization: %s", orgID)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.SYNC_PROFILE_SCHEMA.Code,
			Message:     errors.SYNC_PROFILE_SCHEMA.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	return nil
}

func extractClaimKeyFromURI(uri string) string {
	parts := strings.Split(strings.TrimRight(uri, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func GetProfileSchemaAttributesByScopeAndFilter(orgId, scope string, filters []string) ([]model.ProfileSchemaAttribute, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while initializing DB client for filtering profile schema attributes for org: %s and scope: %s", orgId, scope)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	baseSQL := scripts.FilterProfileSchemaAttributes[provider.NewDBProvider().GetDBType()]
	conditions := []string{}
	args := []interface{}{orgId}
	argID := 2

	for _, f := range filters {
		parts := strings.SplitN(f, " ", 3)
		if len(parts) != 3 {
			continue
		}
		field, operator, value := parts[0], parts[1], parts[2]

		var clause string
		switch operator {
		case "eq":
			clause = fmt.Sprintf("%s = $%d", field, argID)
			args = append(args, value)
		case "co":
			clause = fmt.Sprintf("%s ILIKE $%d", field, argID)
			args = append(args, "%"+value+"%")
		case "sw":
			clause = fmt.Sprintf("%s ILIKE $%d", field, argID)
			args = append(args, value+"%")
		default:
			continue
		}
		conditions = append(conditions, clause)
		argID++
	}

	if len(conditions) > 0 {
		baseSQL += " AND " + strings.Join(conditions, " AND ")
	}

	results, err := dbClient.ExecuteQuery(baseSQL, args...)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to execute profile schema filter query for org: %s and scope: %s", orgId, scope)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
	}

	var attributes []model.ProfileSchemaAttribute
	for _, row := range results {
		attr := mapRowToProfileAttribute(row)
		attributes = append(attributes, attr)
	}

	return attributes, nil
}

func GetProfileSchemaAttributeByMappedLocalClaim(orgId string, claim string) (model.ProfileSchemaAttribute, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while fetching profile schema for org: %s and mapped claim: %s",
			orgId, claim)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return model.ProfileSchemaAttribute{}, serverError
	}
	defer dbClient.Close()

	query := scripts.GetProfileSchemaAttributeByMappedLocalClaim[provider.NewDBProvider().GetDBType()]

	results, err := dbClient.ExecuteQuery(query, orgId, claim)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while fetching profile schema for the org:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.GET_PROFILE_SCHEMA.Code,
			Message:     errors.GET_PROFILE_SCHEMA.Message,
			Description: errorMsg,
		}, err)
		return model.ProfileSchemaAttribute{}, serverError
	}
	if len(results) == 0 {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ATTRIBUTE_NOT_FOUND.Code,
			Message:     errors.ATTRIBUTE_NOT_FOUND.Message,
			Description: "Profile schema attribute not found for org: " + orgId + " and mapped claim : " + claim,
		}, http.StatusNotFound)
		return model.ProfileSchemaAttribute{}, clientError
	}
	row := results[0]
	return mapRowToProfileAttribute(row), nil
}
