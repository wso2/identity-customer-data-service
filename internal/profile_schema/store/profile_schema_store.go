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
	"database/sql"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"strconv"
	"strings"
)

func AddProfileSchemaAttribute(attr model.ProfileSchemaAttribute) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while adding profile schema attribute: %s", attr.AttributeName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.DB_CLIENT_INIT.Code,
			Message:     errors.DB_CLIENT_INIT.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	defer dbClient.Close()

	query := `INSERT INTO profile_schema (org_id, attribute_name, value_type, merge_strategy) 
	          VALUES ($1, $2, $3, $4)`
	_, err = dbClient.ExecuteQuery(query, attr.OrgId, attr.AttributeName, attr.ValueType, attr.MergeStrategy)
	if err != nil {
		errorMsg := fmt.Sprintf("Error occurred while adding profile schema attribute: %s", attr.AttributeName)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors.NewServerError(errors.ErrorMessage{
			Code:        errors.ADD_PROFILE_SCHEMA.Code,
			Message:     errors.ADD_PROFILE_SCHEMA.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	logger.Info("Profile schema attribute added: " + attr.AttributeName)
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

	query := `SELECT attribute_id, org_id, attribute_name, value_type, merge_strategy 
	          FROM profile_schema WHERE org_id = $1 AND attribute_id = $2`

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
		// if it doesnt exists - should we return 404 or empty
		return model.ProfileSchemaAttribute{}, nil
	}

	row := results[0]
	return mapRowToProfileAttribute(row), nil
}

func GetProfileSchema(orgId string) ([]*model.ProfileSchemaAttribute, error) {

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

	query := `SELECT attribute_id, org_id, attribute_name, value_type, merge_strategy 
	          FROM profile_schema WHERE org_id = $1`

	results, err := dbClient.ExecuteQuery(query, orgId)
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

	var schema []*model.ProfileSchemaAttribute
	for _, row := range results {
		attr := mapRowToProfileAttribute(row)
		schema = append(schema, &attr)
	}
	return schema, nil
}

func PatchProfileSchemaAttribute(orgID, attributeName string, updates map[string]interface{}) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		return handleDBError("patching profile schema attribute", err)
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
	args = append(args, orgID, attributeName)

	query := `UPDATE profile_schema SET ` + strings.Join(setClauses, ", ") +
		` WHERE org_id = $` + strconv.Itoa(argIndex) + ` AND attribute_name = $` + strconv.Itoa(argIndex+1)

	_, err = dbClient.ExecuteQuery(query, args...)
	if err != nil {
		return handleDBError("executing update for attribute", err)
	}
	return nil
}

func DeleteProfileSchemaAttribute(orgID, attributeName string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		return handleDBError("deleting profile schema attribute", err)
	}
	defer dbClient.Close()

	query := `DELETE FROM profile_schema WHERE org_id = $1 AND attribute_name = $2`
	_, err = dbClient.ExecuteQuery(query, orgID, attributeName)
	if err != nil {
		return handleDBError("executing delete for attribute", err)
	}
	logger.Info("Profile schema attribute deleted: " + attributeName)
	return nil
}

func DeleteProfileSchema(orgID string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		return handleDBError("deleting profile schema", err)
	}
	defer dbClient.Close()

	query := `DELETE FROM profile_schema WHERE org_id = $1`
	_, err = dbClient.ExecuteQuery(query, orgID)
	if err != nil {
		return handleDBError("executing delete for org", err)
	}
	logger.Info("Profile schema cleared for org: " + orgID)
	return nil
}

// helper: convert DB row to model
func mapRowToProfileAttribute(row map[string]interface{}) model.ProfileSchemaAttribute {
	return model.ProfileSchemaAttribute{
		AttributeId:   fmt.Sprint(row["attribute_id"]),
		OrgId:         row["org_id"].(string),
		AttributeName: row["attribute_name"].(string),
		ValueType:     row["value_type"].(string),
		MergeStrategy: row["merge_strategy"].(string),
	}
}
