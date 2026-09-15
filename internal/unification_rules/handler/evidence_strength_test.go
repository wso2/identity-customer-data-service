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
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

func withOverrideAllowed(t *testing.T, allowed bool) {
	t.Helper()

	conf := config.Config{}
	conf.IdentityResolution.AllowEvidenceStrengthOverride = allowed
	config.OverrideCDSRuntime(conf)
}

// TestEvidenceStrengthDefaultsWhenNotSupplied covers the ordinary path: a rule that says
// nothing about strengths takes the values derived from its attribute type, whether or not
// overrides are permitted.
func TestEvidenceStrengthDefaultsWhenNotSupplied(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		withOverrideAllowed(t, allowed)

		strength, err := resolveEvidenceStrength("", constants.DefaultMatchStrength,
			constants.AttributeTypeEmail, "match_strength")
		if err != nil {
			t.Fatalf("override allowed=%v: unexpected error: %v", allowed, err)
		}
		if strength != constants.EvidenceStrengthHigh {
			t.Errorf("override allowed=%v: EMAIL match_strength = %q, want %q",
				allowed, strength, constants.EvidenceStrengthHigh)
		}
	}
}

// TestEvidenceStrengthRefusedWhenOverrideDisabled is the point of the setting: a supplied
// strength is rejected rather than quietly replaced by the default, so a caller cannot
// believe it configured something the engine is not using.
func TestEvidenceStrengthRefusedWhenOverrideDisabled(t *testing.T) {
	withOverrideAllowed(t, false)

	_, err := resolveEvidenceStrength(constants.EvidenceStrengthLow, constants.DefaultMatchStrength,
		constants.AttributeTypeEmail, "match_strength")
	if err == nil {
		t.Fatal("expected a supplied strength to be refused while overrides are disabled")
	}
}

func TestEvidenceStrengthAcceptedWhenOverrideEnabled(t *testing.T) {
	withOverrideAllowed(t, true)

	strength, err := resolveEvidenceStrength(constants.EvidenceStrengthLow, constants.DefaultMatchStrength,
		constants.AttributeTypeEmail, "match_strength")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strength != constants.EvidenceStrengthLow {
		t.Errorf("match_strength = %q, want %q", strength, constants.EvidenceStrengthLow)
	}
}

func TestEvidenceStrengthRejectsUnknownValue(t *testing.T) {
	withOverrideAllowed(t, true)

	if _, err := resolveEvidenceStrength("VERY_HIGH", constants.DefaultMatchStrength,
		constants.AttributeTypeEmail, "match_strength"); err == nil {
		t.Error("expected an unknown strength to be rejected")
	}
}

// TestEvidenceStrengthFallsBackForUnknownAttributeType guards the defaulting path against a
// missing table entry, which would otherwise store an empty strength.
func TestEvidenceStrengthFallsBackForUnknownAttributeType(t *testing.T) {
	withOverrideAllowed(t, false)

	strength, err := resolveEvidenceStrength("", constants.DefaultMatchStrength,
		"NOT_A_REAL_TYPE", "match_strength")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !constants.AllowedEvidenceStrengths[strength] {
		t.Errorf("fallback produced %q, which is not a valid strength", strength)
	}
}
