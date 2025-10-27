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
	model "github.com/wso2/identity-customer-data-service/internal/admin_config/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func GetAdminConfig(tenantId string) (*model.AdminConfig, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for fetching configurations for the organization: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_ADMIN_CONFIG.Code,
			Message:     errors2.GET_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	query := scripts.GetOrgConfigurations[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, tenantId)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to execute query for fetching configurations for organization: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_ADMIN_CONFIG.Code,
			Message:     errors2.GET_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}

	if len(results) == 0 {
		logger.Warn(fmt.Sprintf("No configurations found for organization: %s", tenantId))
		return nil, nil
	}
	row := results[0]
	config := model.AdminConfig{
		TenantId:              row["tenant_id"].(string),
		CDSEnabled:            row["cds_enabled"].(bool),
		InitialSchemaSyncDone: row["initial_schema_sync_done"].(bool),
	}
	return &config, nil
}

// UpdateAdminConfig updates organization-level admin configuration (e.g., CDS enablement, schema sync flags).
func UpdateAdminConfig(config model.AdminConfig, tenantId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get DB client for updating configurations for tenant: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ADMIN_CONFIG.Code,
			Message:     errors2.UPDATE_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for updating configurations for tenant: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ADMIN_CONFIG.Code,
			Message:     errors2.UPDATE_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}

	query := scripts.UpdateOrgConfigurations[provider.NewDBProvider().GetDBType()]
	_, err = tx.Exec(query, config.CDSEnabled, config.InitialSchemaSyncDone, tenantId)
	if err != nil {
		_ = tx.Rollback()
		errorMsg := fmt.Sprintf("Failed to execute update for configurations for tenant: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ADMIN_CONFIG.Code,
			Message:     errors2.UPDATE_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}

	return tx.Commit()
}

// UpdateInitialSchemaSyncConfig updates organization-level admin configuration (e.g., CDS enablement, schema sync flags).
func UpdateInitialSchemaSyncConfig(state bool, tenantId string) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get DB client for updating configurations for tenant: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ADMIN_CONFIG.Code,
			Message:     errors2.UPDATE_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for updating configurations for tenant: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ADMIN_CONFIG.Code,
			Message:     errors2.UPDATE_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}

	query := scripts.UpdateInitialSchemaSyncDoneConfig[provider.NewDBProvider().GetDBType()]
	_, err = tx.Exec(query, state, tenantId)
	if err != nil {
		_ = tx.Rollback()
		errorMsg := fmt.Sprintf("Failed to execute update for configurations for tenant: %s", tenantId)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_ADMIN_CONFIG.Code,
			Message:     errors2.UPDATE_ADMIN_CONFIG.Message,
			Description: errorMsg,
		}, err)
	}
	return tx.Commit()
}
