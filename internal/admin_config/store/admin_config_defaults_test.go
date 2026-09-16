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

package store

import (
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// TestAdminConfigRowsDefaultAutoMergeEnabled is an upgrade guard.
//
// auto_merge_enabled is stored as a row in cds_config, so an org configured before the key
// existed simply has no row for it. Scanning that absence into the zero value would leave
// automatic merging switched off for every existing tenant — every match becoming a review
// task with nothing in the logs to say why. The absence must read as enabled, and only an
// explicit "false" may turn it off.
func TestAdminConfigRowsDefaultAutoMergeEnabled(t *testing.T) {
	tests := []struct {
		name string
		rows []map[string]interface{}
		want bool
	}{
		{
			name: "org predating the key",
			rows: []map[string]interface{}{
				{"config": constants.ConfigCDSEnabled, "value": "true"},
			},
			want: true,
		},
		{
			name: "explicitly disabled",
			rows: []map[string]interface{}{
				{"config": constants.ConfigCDSEnabled, "value": "true"},
				{"config": constants.ConfigAutoMergeEnabled, "value": "false"},
			},
			want: false,
		},
		{
			name: "explicitly enabled",
			rows: []map[string]interface{}{
				{"config": constants.ConfigAutoMergeEnabled, "value": "true"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := scanAdminConfigRows("acme", tt.rows)
			if config.AutoMergeEnabled != tt.want {
				t.Errorf("AutoMergeEnabled = %v, want %v", config.AutoMergeEnabled, tt.want)
			}
		})
	}
}
