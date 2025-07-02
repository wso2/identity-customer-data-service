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
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strconv"
	"strings"
)

func AddProfileSchemaAttributes(attrs []model.ProfileSchemaAttribute) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()

	if err != nil {
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: "Error initializing DB client for bulk insert",
		}, err)
	}
	defer dbClient.Close()

	if len(attrs) == 0 {
		return nil // nothing to insert
	}

	// Build query
	baseQuery := `INSERT INTO profile_schema (tenant_id, attribute_id, attribute_name, value_type, merge_strategy, application_identifier, mutability, multi_valued, sub_attributes, canonical_values) VALUES `
	valueStrings := make([]string, 0, len(attrs))
	valueArgs := make([]interface{}, 0, len(attrs)*10)

	for i, attr := range attrs {
		idx := i * 10
		log.GetLogger().Info("Adding profile schema attribute: " + attr.AttributeId)
		subAttrsJSON, err := json.Marshal(attr.SubAttributes)
		if err != nil {
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.MARSHAL_JSON.Code,
				Message:     errors.MARSHAL_JSON.Message,
				Description: fmt.Sprintf("Failed to marshal sub attributes for attribute %s", attr.AttributeId),
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

		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d,  $%d, $%d,  $%d, $%d) ", idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7, idx+8, idx+9, idx+10))
		valueArgs = append(valueArgs, attr.OrgId, attr.AttributeId, attr.AttributeName, attr.ValueType,
			attr.MergeStrategy, attr.ApplicationIdentifier, attr.Mutability, attr.MultiValued, subAttrsJSON, canonicalJSON)

	}

	query := baseQuery + strings.Join(valueStrings, ", ")

	_, err = dbClient.ExecuteQuery(query, valueArgs...)
	if err != nil {
		return errors.NewServerError(errors.ErrorMessage{
			Code:        errors.ADD_PROFILE_SCHEMA.Code,
			Message:     errors.ADD_PROFILE_SCHEMA.Message,
			Description: "Failed to insert profile schema attributes in bulk",
		}, err)
	}

	logger.Info(fmt.Sprintf("Bulk added %d profile schema attributes", len(attrs)))
	return nil
}

func GetProfileSchemaAttribute(orgId, attributeId string) (model.ProfileSchemaAttribute, error) {

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

	query := `SELECT attribute_id, attribute_name, value_type, merge_strategy, mutability , application_identifier, multi_valued,   sub_attributes::text,
  canonical_values::text
	          FROM profile_schema WHERE tenant_id = $1 AND attribute_id = $2`

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
			Code:        errors.GET_PROFILE_SCHEMA.Code,
			Message:     errors.GET_PROFILE_SCHEMA.Message,
			Description: "Profile schema attribute not found for org: " + orgId + " and attribute: " + attributeId,
		}, http.StatusNotFound)
		return model.ProfileSchemaAttribute{}, clientError
	}

	row := results[0]
	return mapRowToProfileAttribute(row), nil
}

func GetProfileSchemaAttributes(orgId, scope string) ([]model.ProfileSchemaAttribute, error) {
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

	query := `
		SELECT attribute_id, tenant_id, attribute_name, value_type, merge_strategy, mutability, application_identifier, multi_valued,   sub_attributes::text,
  canonical_values::text
		FROM profile_schema
		WHERE tenant_id = $1 AND attribute_name LIKE $2`
	scopePrefix := scope + ".%" // e.g., identity_attributes.%

	results, err := dbClient.ExecuteQuery(query, orgId, scopePrefix)
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

	query := `SELECT attribute_id, attribute_name, value_type, merge_strategy, mutability , application_identifier, multi_valued,   sub_attributes::text,
  canonical_values::text
	          FROM profile_schema 
	          WHERE tenant_id = $1 AND attribute_name = $2 LIMIT 1`

	results, err := dbClient.ExecuteQuery(query, orgId, attributeName)
	if err != nil {
		if len(results) == 0 {
			logger.Debug(fmt.Sprintf("No profile schema found for attribute: %s in org: %s", attributeName, orgId))
			return nil, nil
		}
		errorMsg := fmt.Sprintf("Failed in fetching schema attribute '%s' for org '%s'", attributeName, orgId)
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
			log.GetLogger().Debug("Failed to unmarshal sub_attributes", log.Error(err))
		}
	}

	var canonicalValues []model.CanonicalValue
	if raw, ok := row["canonical_values"].(string); ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &canonicalValues); err != nil {
			log.GetLogger().Debug("Failed to unmarshal canonical_values", log.Error(err))
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

	logger.Info(fmt.Sprintf("Successfully fetched profile schema attribute '%s' for org '%s'", attributeName, orgId))
	return attr, nil
}

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

	query := `SELECT attribute_id, attribute_name, value_type, merge_strategy , application_identifier, mutability, multi_valued, sub_attributes::text,
  canonical_values::text
	          FROM profile_schema WHERE tenant_id = $1`

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

func PatchProfileSchemaAttribute(orgId, attributeId string, updates map[string]interface{}) error {
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

func DeleteProfileSchemaAttribute(orgId, attributeId string) error {
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

	query := `DELETE FROM profile_schema WHERE tenant_id = $1 AND attribute_id = $2`
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
	logger.Info("Profile schema attribute deleted: " + attributeId)
	return nil
}

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

	scope = scope + ".%" // e.g., identity_attributes.%

	query := `DELETE FROM profile_schema WHERE tenant_id = $1 AND attribute_name LIKE  $2`
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
	logger.Info(fmt.Sprintf("Profile schema attributes with scope:%s deleted: ", scope))
	return nil
}

func PatchProfileSchemaAttributes(orgId string, updates []model.ProfileSchemaAttribute) error {
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
		return errors.NewServerError(errors.ErrorMessage{
			Code:        "errors..Code",
			Message:     "Failed to begin transaction",
			Description: err.Error(),
		}, err)
	}

	stmt := `
		UPDATE profile_schema
		SET attribute_name = $1,
			value_type = $2,
			merge_strategy = $3,
			mutability = $4,
			application_identifier = $5,
			multi_valued = $6,
			canonical_values = $7,
			sub_attributes = $8
		WHERE tenant_id = $9 AND attribute_id = $10
	`

	for _, attr := range updates {
		subAttrsJSON, err := json.Marshal(attr.SubAttributes)
		if err != nil {
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.MARSHAL_JSON.Code,
				Message:     errors.MARSHAL_JSON.Message,
				Description: fmt.Sprintf("Failed to marshal sub attributes for attribute %s", attr.AttributeId),
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
		}

		if _, err := tx.Exec(stmt, args...); err != nil {
			tx.Rollback()
			logger.Debug("Failed to update attribute: "+attr.AttributeId, log.Error(err))
			return errors.NewServerError(errors.ErrorMessage{
				Code:        errors.EXECUTE_QUERY.Code,
				Message:     "Failed to update profile schema attribute",
				Description: err.Error(),
			}, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.NewServerError(errors.ErrorMessage{
			Code:        "rrors.Code",
			Message:     "Failed to commit transaction",
			Description: err.Error(),
		}, err)
	}

	logger.Info(fmt.Sprintf("Bulk update completed for %d profile schema attributes (org: %s)", len(updates), orgId))
	return nil
}

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

	query := `DELETE FROM profile_schema WHERE tenant_id = $1`
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

// helper: convert DB row to model
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
	//logger := log.GetLogger()
	if err != nil {
		return fmt.Errorf("failed to get DB client: %w", err)
	}
	defer dbClient.Close()

	// Step 1: Delete existing identity_attributes for this org
	deleteQuery := `DELETE FROM profile_schema WHERE tenant_id = $1 AND attribute_name LIKE 'identity_attributes.%'`
	if _, err := dbClient.ExecuteQuery(deleteQuery, orgID); err != nil {
		return fmt.Errorf("failed to delete existing identity_attributes: %w", err)
	}

	// Step 2: Insert new attributes
	insertQuery := `INSERT INTO profile_schema 
	(tenant_id, attribute_id, attribute_name, value_type, merge_strategy, mutability, application_identifier, multi_valued, canonical_values, sub_attributes, scim_dialect) 
	VALUES `

	var valueStrings []string
	var valueArgs []interface{}
	argIndex := 1

	for _, attr := range attrs {
		canonicalJSON, _ := json.Marshal(attr.CanonicalValues)
		subAttrJSON, _ := json.Marshal(attr.SubAttributes)
		attrKey := extractClaimKeyFromURI(attr.AttributeName)
		attr.AttributeName = attrKey

		valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5, argIndex+6,
			argIndex+7, argIndex+8, argIndex+9, argIndex+10))
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
		)
		argIndex += 11
	}

	insertQuery += strings.Join(valueStrings, ",")
	if _, err := dbClient.ExecuteQuery(insertQuery, valueArgs...); err != nil {
		return fmt.Errorf("failed to insert profile schema attributes: %w", err)
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
