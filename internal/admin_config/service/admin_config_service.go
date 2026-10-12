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
	"context"
	"fmt"
	"net/http"

	"github.com/wso2/identity-customer-data-service/internal/admin_config/model"
	"github.com/wso2/identity-customer-data-service/internal/admin_config/store"
	appProvider "github.com/wso2/identity-customer-data-service/internal/application/provider"
	consentService "github.com/wso2/identity-customer-data-service/internal/consent/service"
	orgService "github.com/wso2/identity-customer-data-service/internal/organization/service"
	"github.com/wso2/identity-customer-data-service/internal/organization/suborg"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/service"
	sysconfig "github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
)

// AdminConfigServiceInterface defines the service interface.
type AdminConfigServiceInterface interface {
	GetAdminConfig(ctx context.Context, orgHandle string) (model.AdminConfig, error)
	IsCDSEnabled(ctx context.Context, orgHandle string) bool
	IsInitialSchemaSyncDone(ctx context.Context, orgHandle string) bool
	IsSystemApplication(ctx context.Context, orgHandle, appId string) (bool, error)
	UpdateAdminConfig(ctx context.Context, category model.AdminConfig, orgHandle string) error
	UpdateInitialSchemaSync(ctx context.Context, state bool, orgHandle string) error
}

// AdminConfigService is the default implementation.
type AdminConfigService struct{}

// IsCDSEnabled reports whether CDS is enabled for the org. A sub org is enabled when its primary
// org enabled CDS and the organization access of the primary org reaches the sub org. An org that
// CDS does not know is not enabled.
func (a AdminConfigService) IsCDSEnabled(ctx context.Context, orgHandle string) bool {
	config, err := store.GetAdminConfig(ctx, orgHandle)
	if err == nil && config != nil && config.CDSEnabled {
		return true
	}
	primaryHandle, known := orgService.PrimaryHandleOf(ctx, orgHandle)
	if !known || primaryHandle == orgHandle {
		return false
	}
	primaryConfig, err := store.GetAdminConfig(ctx, primaryHandle)
	if err != nil || primaryConfig == nil || !primaryConfig.CDSEnabled {
		return false
	}
	return orgService.IsSubOrgEnabled(ctx, orgHandle)
}

func (a AdminConfigService) IsInitialSchemaSyncDone(ctx context.Context, orgHandle string) bool {
	config, err := store.GetAdminConfig(ctx, orgHandle)
	if err != nil || config == nil {
		return false
	}
	return config.InitialSchemaSyncDone
}

func (a AdminConfigService) IsSystemApplication(ctx context.Context, orgHandle, appId string) (bool, error) {
	config, err := store.GetAdminConfig(ctx, orgHandle)
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

func (a AdminConfigService) GetAdminConfig(ctx context.Context, orgHandle string) (model.AdminConfig, error) {

	defaultConfig := model.AdminConfig{
		OrgHandle:             orgHandle,
		CDSEnabled:            false,
		InitialSchemaSyncDone: false,
		SystemApplications:    []string{},
	}
	config, err := store.GetAdminConfig(ctx, orgHandle)
	if err != nil {
		return defaultConfig, err
	}
	if config == nil {
		config = &defaultConfig
	}
	// A sub org shows the enablement of its primary org.
	if !config.CDSEnabled {
		config.CDSEnabled = a.IsCDSEnabled(ctx, orgHandle)
	}
	return *config, nil
}

func (a AdminConfigService) UpdateAdminConfig(ctx context.Context,
	updatedConfig model.AdminConfig, orgHandle string) error {
	// A sub org inherits the configuration of its primary org and cannot change it.
	if err := suborg.RejectWrite(ctx, orgHandle); err != nil {
		return err
	}
	isCDSEnabledInitialState := a.IsCDSEnabled(ctx, orgHandle)
	isInitialSchemaSyncDoneInitialState := a.IsInitialSchemaSyncDone(ctx, orgHandle)

	// Schema sync status should not be changed via this method.
	updatedConfig.InitialSchemaSyncDone = isInitialSchemaSyncDoneInitialState

	schemaService := service.GetProfileSchemaService()
	if !isCDSEnabledInitialState && !isInitialSchemaSyncDoneInitialState && updatedConfig.CDSEnabled {
		// CDS is being enabled for the first time. Trigger initial schema sync.
		err := schemaService.SyncProfileSchema(ctx, orgHandle)
		if err != nil {
			return err
		}
		// Seed the mandatory "Identity Data" consent category after schema is ready.
		if err := consentService.GetConsentCategoryService().SeedDefaultConsentCategory(ctx, orgHandle); err != nil {
			return err
		}
		updatedConfig.InitialSchemaSyncDone = true
	}

	// In app_id mode, register each system application so its clientId->app_id mapping exists for the GET path.
	if sysconfig.GetCDSRuntime().Config.UsesAppIDIdentifier() {
		appService := appProvider.NewApplicationProvider().GetApplicationService()
		for _, appID := range updatedConfig.SystemApplications {
			exists, err := appService.ResolveAndRegisterApplication(ctx, appID, orgHandle)
			if err != nil {
				return err
			}
			if !exists {
				return errors.NewClientError(errors.ErrorMessage{
					Code:        errors.UPDATE_CONFIG_BAD_REQUEST.Code,
					Message:     errors.UPDATE_CONFIG_BAD_REQUEST.Message,
					Description: fmt.Sprintf("System application '%s' does not exist in the identity server.", appID),
				}, http.StatusBadRequest)
			}
		}
	}

	if err := store.UpdateAdminConfig(ctx, updatedConfig, orgHandle); err != nil {
		return err
	}

	// When a primary org disables CDS, the org events stop. The org tree is synced again after the
	// next enable.
	if isCDSEnabledInitialState && !updatedConfig.CDSEnabled {
		if err := orgService.ResetTreeSync(ctx, orgHandle); err != nil {
			return err
		}
	}

	// When a primary org enables CDS, CDS stores the primary org. The sub orgs are stored when the
	// primary org syncs its org tree.
	if !isCDSEnabledInitialState && updatedConfig.CDSEnabled {
		if _, err := orgService.EnsurePrimary(ctx, orgHandle); err != nil {
			return err
		}
	}
	return nil
}

func (a AdminConfigService) UpdateInitialSchemaSync(ctx context.Context, state bool, orgHandle string) error {

	return store.UpdateInitialSchemaSyncConfig(ctx, state, orgHandle)
}

// GetAdminConfigService returns a new instance.
func GetAdminConfigService() AdminConfigServiceInterface {
	return &AdminConfigService{}
}
