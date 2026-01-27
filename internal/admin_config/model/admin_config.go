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

type AdminConfig struct {
	TenantId              string   `json:"tenant_id" bson:"tenant_id"`
	CDSEnabled            bool     `json:"cds_enabled" bson:"cds_enabled"`
	InitialSchemaSyncDone bool     `json:"initial_schema_sync_done" bson:"initial_schema_sync_done"`
	SystemApplications    []string `json:"system_applications" bson:"system_applications"`
}

type AdminConfigAPI struct {
	CDSEnabled         bool     `json:"cds_enabled" bson:"cds_enabled"`
	SystemApplications []string `json:"system_applications,omitempty" bson:"system_applications,omitempty"`
}

type AdminConfigUpdateAPI struct {
	CDSEnabled         *bool    `json:"cds_enabled" bson:"cds_enabled"`
	SystemApplications []string `json:"system_applications,omitempty"`
}
