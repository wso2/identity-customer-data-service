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

package model

import "encoding/json"

// ApplicationData represents contextual data for an application
type ApplicationData struct {
	AppId           string                 `json:"application_id" bson:"application_id"`
	Devices         []Devices              `json:"devices,omitempty" bson:"devices,omitempty"`
	AppSpecificData map[string]interface{} `json:"app_specific_data,omitempty" bson:"app_specific_data,omitempty"`
}

// Devices represents user devices
type Devices struct {
	DeviceId       string `json:"device_id,omitempty" bson:"device_id,omitempty"`
	DeviceType     string `json:"device_type,omitempty" bson:"device_type,omitempty"`
	LastUsed       int    `json:"last_used,omitempty" bson:"last_used,omitempty"`
	Os             string `json:"os,omitempty" bson:"os,omitempty"`
	Browser        string `json:"browser,omitempty" bson:"browser,omitempty"`
	BrowserVersion string `json:"browser_version,omitempty" bson:"browser_version,omitempty"`
	Ip             string `json:"ip,omitempty" bson:"ip,omitempty"`
	Region         string `json:"region,omitempty" bson:"region,omitempty"`
}

func (a ApplicationData) MarshalJSON() ([]byte, error) {
	base := map[string]interface{}{
		"application_id": a.AppId,
		"devices":        a.Devices,
	}
	for k, v := range a.AppSpecificData {
		base[k] = v
	}
	return json.Marshal(base)
}
