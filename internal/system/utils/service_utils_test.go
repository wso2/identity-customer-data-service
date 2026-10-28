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

package utils

import (
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

func TestBuildProfileLocation(t *testing.T) {
	_ = config.InitializeCDSRuntime("", &config.Config{ServerURL: "https://localhost:8900/"})

	cases := []struct {
		name      string
		orgHandle string
		want      string
	}{
		{"tenant handle", "vanheim",
			"https://localhost:8900/t/vanheim/cds/api/v1/profiles/p1"},
		{"sub-org uuid", "9133fc6c-2e8e-472e-bf99-86816c1cba2f",
			"https://localhost:8900/o/9133fc6c-2e8e-472e-bf99-86816c1cba2f/cds/api/v1/profiles/p1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BuildProfileLocation(c.orgHandle, "p1"); got != c.want {
				t.Errorf("BuildProfileLocation(%q) = %q, want %q", c.orgHandle, got, c.want)
			}
		})
	}
}
