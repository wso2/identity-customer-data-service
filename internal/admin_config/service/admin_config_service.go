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
	IsSystemApplication(tenantId, appId string) (bool, error)
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

func (a AdminConfigService) IsSystemApplication(tenantId, appId string) (bool, error) {
    config, err := store.GetAdminConfig(tenantId)
    if err != nil {
        return false, err
    }
    if config == nil {
        return false, nil
    }
    for _, sysApp := range config.SystemApplications {
        if sysApp == appId {
            return true, nil
        }
    }
    return false, nil
}

func (a AdminConfigService) GetAdminConfig(tenantId string) (model.AdminConfig, error) {

	defaultConfig := model.AdminConfig{
		TenantId:              tenantId,
		CDSEnabled:            false,
		InitialSchemaSyncDone: false,
		SystemApplications:    []string{},
	}
	config, err := store.GetAdminConfig(tenantId)
	if err != nil || config == nil {
		return defaultConfig, err
	}
	return *config, nil
}

func (a AdminConfigService) UpdateAdminConfig(updatedConfig model.AdminConfig, orgHandle string) error {
	isCDSEnabledInitialState := a.IsCDSEnabled(orgHandle)
	isInitialSchemaSyncDoneInitialState := a.IsInitialSchemaSyncDone(orgHandle)
  
	// Schema sync status should not be changed via this method.
	updatedConfig.InitialSchemaSyncDone = isInitialSchemaSyncDoneInitialState

	schemaService := service.GetProfileSchemaService()
	if !isCDSEnabledInitialState && !isInitialSchemaSyncDoneInitialState && updatedConfig.CDSEnabled {
		// CDS is being enabled for the first time. Trigger initial schema sync.
		err := schemaService.SyncProfileSchema(orgHandle)
		if err != nil {
			return err
		}
		updatedConfig.InitialSchemaSyncDone = true
	}

	return store.UpdateAdminConfig(updatedConfig, orgHandle)
}

func (a AdminConfigService) UpdateInitialSchemaSync(state bool, tenantId string) error {

	return store.UpdateInitialSchemaSyncConfig(state, tenantId)
}

// GetAdminConfigService returns a new instance.
func GetAdminConfigService() AdminConfigServiceInterface {
	return &AdminConfigService{}
}
