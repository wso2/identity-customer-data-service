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
	"github.com/wso2/identity-customer-data-service/internal/admin_config/model"
	"github.com/wso2/identity-customer-data-service/internal/admin_config/store"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
)

// AdminConfigServiceInterface defines the service interface.
type AdminConfigServiceInterface interface {
	GetAdminConfig(tenantId string) (model.AdminConfig, error)
	IsCDSEnabled(tenantId string) bool
	IsInitialSchemaSyncDone(tenantId string) bool
	UpdateAdminConfig(category model.AdminConfig, tenantId string) error
	UpdateInitialSchemaSync(state bool, tenantId string) error
}

// AdminConfigService is the default implementation.
type AdminConfigService struct{}

func (a AdminConfigService) IsCDSEnabled(tenantId string) bool {
	config, err := store.GetAdminConfig(tenantId)
	if err != nil || config == nil {
		return false
	}
	return config.CDSEnabled
}

func (a AdminConfigService) IsInitialSchemaSyncDone(tenantId string) bool {
	config, err := store.GetAdminConfig(tenantId)
	if err != nil || config == nil {
		return false
	}
	return config.InitialSchemaSyncDone
}

func (a AdminConfigService) GetAdminConfig(tenantId string) (model.AdminConfig, error) {

	defaultConfig := model.AdminConfig{
		TenantId:              tenantId,
		CDSEnabled:            false,
		InitialSchemaSyncDone: false,
	}
	config, err := store.GetAdminConfig(tenantId)
	if err != nil || config == nil {
		return defaultConfig, err
	}
	return *config, nil
}

func (a AdminConfigService) UpdateAdminConfig(updatedConfig model.AdminConfig, orgHandle string) error {

	isCDSEnabledInitialState := a.IsCDSEnabled(orgHandle)
	isIsInitialSchemaSyncDoneInitialState := a.IsInitialSchemaSyncDone(orgHandle)
	// Schema sync status should not be changed via this method.
	updatedConfig.InitialSchemaSyncDone = isIsInitialSchemaSyncDoneInitialState
	schemaService := service.GetProfileSchemaService()
	if !isCDSEnabledInitialState && !isIsInitialSchemaSyncDoneInitialState && updatedConfig.CDSEnabled {
		// CDS is being enabled for the first time. Trigger initial schema sync.
		err := schemaService.SyncProfileSchema(orgHandle)
		if err != nil {
			return err
		}
		updatedConfig.InitialSchemaSyncDone = true
		err = store.UpdateAdminConfig(updatedConfig, orgHandle)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a AdminConfigService) UpdateInitialSchemaSync(state bool, tenantId string) error {

	return store.UpdateInitialSchemaSyncConfig(state, tenantId)
}

// GetAdminConfigService returns a new instance.
func GetAdminConfigService() AdminConfigServiceInterface {
	return &AdminConfigService{}
}
