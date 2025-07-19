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

package schedulers

import (
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/client"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// StartSchemaFetchScheduler starts the periodic schema fetch job.
func StartSchemaFetchScheduler(idClient *client.IdentityClient, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once at startup
	//todo: check if we need this and ofcz we cant hardcode "carbon.super" or "wso2conasia"
	fetchSchemas(idClient, "wso2conasia")

	for range ticker.C {
		fetchSchemas(idClient, "wso2conasia")
	}
}

// fetchSchemas gets the schema from the identity server and updates local schema
func fetchSchemas(idClient *client.IdentityClient, orgID string) {
	logger := log.GetLogger()

	claims, err := idClient.GetProfileSchema(orgID)
	if err != nil {
		logger.Error("Failed to fetch profile schema from identity server", log.Error(err))
		return
	}

	if len(claims) > 0 {
		err := store.UpsertIdentityAttributes(orgID, claims)
		if err != nil {
			logger.Error("Failed to store fetched profile schema", log.Error(err))
		} else {
			logger.Info("Profile schema successfully updated for org: " + orgID)
		}
	}
}
