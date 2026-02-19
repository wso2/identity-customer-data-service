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

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterApplicationData_IncludeAppDataFalse(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"key": "value"},
	}
	result := FilterApplicationData(appData, "app1", false, ApplicationDataFilterParams{IncludeAppData: false})
	assert.Empty(t, result, "expected empty map when IncludeAppData is false")
}

func TestFilterApplicationData_EmptyAppData(t *testing.T) {
	result := FilterApplicationData(nil, "app1", false, ApplicationDataFilterParams{IncludeAppData: true})
	assert.Empty(t, result, "expected empty map when appData is nil")

	result = FilterApplicationData(map[string]map[string]interface{}{}, "app1", true, ApplicationDataFilterParams{IncludeAppData: true})
	assert.Empty(t, result, "expected empty map when appData is empty")
}

func TestFilterApplicationData_SystemApp_NoRequestedAppIDs_ReturnsAll(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
		"app2": {"k2": "v2"},
	}
	result := FilterApplicationData(appData, "", true, ApplicationDataFilterParams{
		IncludeAppData:  true,
		RequestedAppIDs: nil,
	})
	assert.Equal(t, appData, result, "system app with no filter should return all app data")
}

func TestFilterApplicationData_SystemApp_WildcardAppID_ReturnsAll(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
		"app2": {"k2": "v2"},
	}
	result := FilterApplicationData(appData, "", true, ApplicationDataFilterParams{
		IncludeAppData:  true,
		RequestedAppIDs: []string{"*"},
	})
	assert.Equal(t, appData, result, "system app with wildcard should return all app data")
}

func TestFilterApplicationData_SystemApp_SpecificAppIDs(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
		"app2": {"k2": "v2"},
		"app3": {"k3": "v3"},
	}
	result := FilterApplicationData(appData, "", true, ApplicationDataFilterParams{
		IncludeAppData:  true,
		RequestedAppIDs: []string{"app1", "app3"},
	})
	assert.Len(t, result, 2, "expected 2 app entries")
	assert.Contains(t, result, "app1")
	assert.Contains(t, result, "app3")
	assert.NotContains(t, result, "app2")
}

func TestFilterApplicationData_SystemApp_RequestedAppIDNotPresent(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
	}
	result := FilterApplicationData(appData, "", true, ApplicationDataFilterParams{
		IncludeAppData:  true,
		RequestedAppIDs: []string{"app99"},
	})
	assert.Empty(t, result, "expected empty when requested app ID does not exist")
}

func TestFilterApplicationData_RegularApp_CallerAppIDMatch(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
		"app2": {"k2": "v2"},
	}
	result := FilterApplicationData(appData, "app1", false, ApplicationDataFilterParams{IncludeAppData: true})
	assert.Len(t, result, 1)
	assert.Contains(t, result, "app1")
	assert.Equal(t, map[string]interface{}{"k1": "v1"}, result["app1"])
}

func TestFilterApplicationData_RegularApp_CallerAppIDNoMatch(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
	}
	result := FilterApplicationData(appData, "app99", false, ApplicationDataFilterParams{IncludeAppData: true})
	assert.Empty(t, result, "expected empty when callerAppID does not match any app data")
}

func TestFilterApplicationData_RegularApp_EmptyCallerAppID(t *testing.T) {
	appData := map[string]map[string]interface{}{
		"app1": {"k1": "v1"},
	}
	result := FilterApplicationData(appData, "", false, ApplicationDataFilterParams{IncludeAppData: true})
	assert.Empty(t, result, "expected empty when callerAppID is empty")
}
