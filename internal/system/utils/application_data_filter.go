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

package utils

import (
	"net/http"
	"strings"
)

type ApplicationDataFilterParams struct {
	IncludeAppData  bool     // application_data=true
	RequestedAppIDs []string // application_id=app1,app2 or * for all
}

func ParseApplicationDataParams(r *http.Request) ApplicationDataFilterParams {
	params := ApplicationDataFilterParams{
		IncludeAppData:  false,
		RequestedAppIDs: []string{},
	}

	appDataParam := r.URL.Query().Get("application_data")
	if strings.ToLower(appDataParam) == "true" {
		params.IncludeAppData = true
	}

	appIDParam := r.URL.Query().Get("application_id")
	if appIDParam != "" {
		if appIDParam == "*" {
			params.RequestedAppIDs = []string{"*"}
		} else {
			ids := strings.Split(appIDParam, ",")
			for _, id := range ids {
				trimmed := strings.TrimSpace(id)
				if trimmed != "" {
					params.RequestedAppIDs = append(params.RequestedAppIDs, trimmed)
				}
			}
		}
	}

	return params
}

func FilterApplicationData(
	appData map[string]map[string]interface{},
	callerAppID string,
	isSystemApp bool,
	params ApplicationDataFilterParams,
) map[string]map[string]interface{} {

	if !params.IncludeAppData {
		return make(map[string]map[string]interface{})
	}

	if appData == nil || len(appData) == 0 {
		return make(map[string]map[string]interface{})
	}

	if isSystemApp {
		return filterForSystemApp(appData, params.RequestedAppIDs)
	}

	return filterForRegularApp(appData, callerAppID)
}

func filterForSystemApp(appData map[string]map[string]interface{}, requestedAppIDs []string) map[string]map[string]interface{} {
	if len(requestedAppIDs) == 0 {
		// If application_data=true but no application_id specified,
		// return all data for system apps
		return appData
	}

	for _, id := range requestedAppIDs {
		if id == "*" {
			return appData
		}
	}

	filtered := make(map[string]map[string]interface{})
	for _, appID := range requestedAppIDs {
		if data, exists := appData[appID]; exists {
			filtered[appID] = data
		}
	}

	return filtered
}

func filterForRegularApp(appData map[string]map[string]interface{}, callerAppID string) map[string]map[string]interface{} {
	filtered := make(map[string]map[string]interface{})

	if callerAppID != "" {
		if data, exists := appData[callerAppID]; exists {
			filtered[callerAppID] = data
		}
	}

	return filtered
}
