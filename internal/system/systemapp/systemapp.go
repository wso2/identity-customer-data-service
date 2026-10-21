/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

// Package systemapp holds the credentials of the Identity Server application CDS
// calls the management APIs with, one per organisation.
//
// An OAuth client in the Identity Server belongs to one tenant and can only
// authenticate at that tenant's token endpoint, so a single configured pair
// cannot serve more than one organisation. A deployment whose applications are
// provisioned for it keeps that single pair and stores nothing here; a deployment
// that provisions its own gets a row per organisation, written as that
// organisation enables CDS.
package systemapp

import (
	"context"
	"fmt"
	"sync"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/secrets"
)

// Credentials is one organisation's system application.
type Credentials struct {
	OrgHandle string
	// AppID is the Identity Server application resource ID, kept so that a later
	// run can reach the application it provisioned without searching by name.
	AppID        string
	ClientID     string
	ClientSecret string
}

var (
	sealerOnce   sync.Once
	cachedSealer *secrets.Sealer
	sealerErr    error
)

// sealer builds the deployment's sealer once. The configuration is fixed for the
// life of the process, so the key is resolved once and the AES key schedule is
// not redone on every token fetch.
func sealer() (*secrets.Sealer, error) {

	sealerOnce.Do(func() {
		cachedSealer, sealerErr = secrets.NewSealer(
			config.GetCDSRuntime().Config.AuthServer.SystemApplication.EncryptionKey)
	})
	return cachedSealer, sealerErr
}

// ValidateConfig reports whether the deployment can do what it is configured to
// do. It is called at start so that a deployment which provisions its own
// applications but cannot seal their secrets fails there, rather than at the
// first organisation that tries to enable CDS.
func ValidateConfig(authServer config.AuthServerConfig) error {

	if !authServer.SystemApplication.AutoProvision {
		return nil
	}
	if _, err := secrets.NewSealer(authServer.SystemApplication.EncryptionKey); err != nil {
		return fmt.Errorf(
			"auth_server.system_application.auto_provision is on, so "+
				"auth_server.system_application.encryption_key has to hold a usable key: %w", err)
	}
	if authServer.AdminUsername == "" || authServer.AdminPassword == "" {
		return fmt.Errorf("auth_server.system_application.auto_provision is on, so " +
			"auth_server.admin_username and auth_server.admin_password have to be set: " +
			"an organisation has no application to get a token with until one has been provisioned")
	}
	return nil
}

// Get returns the credentials provisioned for orgHandle. found is false when the
// organisation has none, which is the ordinary state of a deployment that does
// not provision its own applications.
func Get(ctx context.Context, orgHandle string) (Credentials, bool, error) {

	logger := log.GetLogger()

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return Credentials{}, false, getFailed(orgHandle, "failed to get a database client", err)
	}
	defer dbClient.Close()

	results, err := dbClient.ExecuteQueryContext(ctx, scripts.GetOrgSystemApplication, orgHandle)
	if err != nil {
		return Credentials{}, false, getFailed(orgHandle, "failed to read the credentials", err)
	}
	if len(results) == 0 {
		return Credentials{}, false, nil
	}

	row := results[0]
	sealed, _ := row["client_secret"].(string)
	sealedUnder, _ := row["key_id"].(string)

	keeper, err := sealer()
	if err != nil {
		return Credentials{}, false, getFailed(orgHandle, "no usable encryption key is configured", err)
	}
	clientSecret, err := keeper.Open(sealed, sealedUnder)
	if err != nil {
		return Credentials{}, false, getFailed(orgHandle, "failed to open the sealed client secret", err)
	}

	appID, _ := row["app_id"].(string)
	clientID, _ := row["client_id"].(string)
	logger.Debug(fmt.Sprintf("Resolved the system application of organization: %s", orgHandle))

	return Credentials{
		OrgHandle:    orgHandle,
		AppID:        appID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}, true, nil
}

// Save seals the client secret and persists the credentials, replacing whatever
// the organisation held before. Re-provisioning therefore needs no delete.
func Save(ctx context.Context, creds Credentials) error {

	logger := log.GetLogger()

	keeper, err := sealer()
	if err != nil {
		return saveFailed(creds.OrgHandle, "no usable encryption key is configured", err)
	}
	sealed, err := keeper.Seal(creds.ClientSecret)
	if err != nil {
		return saveFailed(creds.OrgHandle, "failed to seal the client secret", err)
	}

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return saveFailed(creds.OrgHandle, "failed to get a database client", err)
	}
	defer dbClient.Close()

	_, err = dbClient.ExecuteQueryContext(ctx, scripts.UpsertOrgSystemApplication,
		creds.OrgHandle, creds.AppID, creds.ClientID, sealed, keeper.KeyID())
	if err != nil {
		return saveFailed(creds.OrgHandle, "failed to persist the credentials", err)
	}

	logger.Info(fmt.Sprintf("Persisted the system application credentials of organization: %s", creds.OrgHandle))
	return nil
}

func getFailed(orgHandle, what string, err error) error {

	errorMsg := fmt.Sprintf("%s for the organization: %s", what, orgHandle)
	log.GetLogger().Debug(errorMsg, log.Error(err))
	return errors.NewServerError(errors.ErrorMessage{
		Code:        errors.GET_SYSTEM_APPLICATION_FAILED.Code,
		Message:     errors.GET_SYSTEM_APPLICATION_FAILED.Message,
		Description: errorMsg,
	}, err)
}

func saveFailed(orgHandle, what string, err error) error {

	errorMsg := fmt.Sprintf("%s for the organization: %s", what, orgHandle)
	log.GetLogger().Debug(errorMsg, log.Error(err))
	return errors.NewServerError(errors.ErrorMessage{
		Code:        errors.SAVE_SYSTEM_APPLICATION_FAILED.Code,
		Message:     errors.SAVE_SYSTEM_APPLICATION_FAILED.Message,
		Description: errorMsg,
	}, err)
}
