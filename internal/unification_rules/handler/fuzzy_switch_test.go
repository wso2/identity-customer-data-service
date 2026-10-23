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

package handler

import (
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

func withFuzzyMatching(t *testing.T, setting *bool) {
	t.Helper()

	conf := config.Config{}
	conf.IdentityResolution.FuzzyMatching = setting
	config.OverrideCDSRuntime(conf)
}

// TestFuzzyMatchingDefaultsToOn pins the upgrade behaviour. The setting is a kill switch on
// a build that carries the feature, so a deployment.yaml that has never heard of it must
// keep matching exactly as the release intends.
func TestFuzzyMatchingDefaultsToOn(t *testing.T) {
	withFuzzyMatching(t, nil)

	if !fuzzyMatchingAllowed() {
		t.Error("an absent fuzzy_matching_enabled must read as enabled")
	}
}

func TestFuzzyMatchingHonoursAnExplicitSetting(t *testing.T) {
	for _, want := range []bool{true, false} {
		setting := want
		withFuzzyMatching(t, &setting)

		if got := fuzzyMatchingAllowed(); got != want {
			t.Errorf("fuzzy_matching_enabled=%v read as %v", want, got)
		}
	}
}

// TestFuzzyIsRefusedNotDowngraded is the point of refusing rather than quietly storing the
// rule as exact: a caller that believes it configured a rule to tolerate typos, and is
// overruled without being told, would misread every merge that rule takes part in.
func TestFuzzyIsRefusedNotDowngraded(t *testing.T) {
	off := false
	withFuzzyMatching(t, &off)

	if err := refuseFuzzy(); err == nil {
		t.Fatal("expected a refusal for fuzzy while the server has it switched off")
	}
}
