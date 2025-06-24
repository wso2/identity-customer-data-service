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
	baseQuery := `INSERT INTO profile_schema (tenant_id, attribute_id, attribute_name, value_type, merge_strategy, application_identifier, mutability) VALUES `
	valueStrings := make([]string, 0, len(attrs))
	valueArgs := make([]interface{}, 0, len(attrs)*7)

	for i, attr := range attrs {
		idx := i * 7
		log.GetLogger().Info("Adding profile schema attribute: " + attr.AttributeId)
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d,  $%d) ", idx+1, idx+2, idx+3, idx+4, idx+5, idx+6, idx+7))
		valueArgs = append(valueArgs, attr.OrgId, attr.AttributeId, attr.AttributeName, attr.ValueType,
			attr.MergeStrategy, attr.ApplicationIdentifier, attr.Mutability)

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

	query := `SELECT attribute_id, attribute_name, value_type, merge_strategy, mutability 
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
		SELECT attribute_id, tenant_id, attribute_name, value_type, merge_strategy, mutability, application_identifier
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
		return nil, errors.NewClientError(errors.ErrorMessage{
			Code:        errors.GET_PROFILE_SCHEMA.Code,
			Message:     errors.GET_PROFILE_SCHEMA.Message,
			Description: fmt.Sprintf("No profile schema attributes found for org: %s and scope: %s", orgId, scope),
		}, http.StatusNotFound)
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

	query := `SELECT attribute_id, attribute_name, value_type, merge_strategy, mutability 
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
	attr := &model.ProfileSchemaAttribute{
		OrgId:         orgId,
		AttributeName: row["attribute_name"].(string),
		ValueType:     row["value_type"].(string),
		MergeStrategy: row["merge_strategy"].(string),
		Mutability:    row["mutability"].(string),
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

	query := `SELECT attribute_id, attribute_name, value_type, merge_strategy , application_identifier, mutability
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
		errorMsg := fmt.Sprintf("Error occurred while patching profile schema for org: %s and attribute: %s", orgId,
			attributeId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1
	for key, value := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIndex))
		args = append(args, value)
		argIndex++
	}
	args = append(args, orgId, attributeId)

	query := `UPDATE profile_schema SET ` + strings.Join(setClauses, ", ") +
		` WHERE tenant_id = $` + strconv.Itoa(argIndex) + ` AND attribute_id = $` + strconv.Itoa(argIndex+1)

	_, err = dbClient.ExecuteQuery(query, args...)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while fetching profile schema for org: %s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.EXECUTE_QUERY.Code,
			Message:     errors.EXECUTE_QUERY.Message,
			Description: errorMsg,
		}, err)
		return serverError
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

	query := `DELETE FROM profile_schema WHERE tenant_id = $1 AND attribute_name LIKE = $2`
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
			application_identifier = $5
		WHERE tenant_id = $6 AND attribute_id = $7
	`

	for _, attr := range updates {
		args := []interface{}{
			attr.AttributeName,
			attr.ValueType,
			attr.MergeStrategy,
			attr.Mutability,
			attr.ApplicationIdentifier,
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
	return model.ProfileSchemaAttribute{
		AttributeId: fmt.Sprint(row["attribute_id"]),
		//OrgId:                 row["tenant_id"].(string),
		AttributeName:         row["attribute_name"].(string),
		ValueType:             row["value_type"].(string),
		MergeStrategy:         row["merge_strategy"].(string),
		Mutability:            row["mutability"].(string),
		ApplicationIdentifier: row["application_identifier"].(string),
	}
}
