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

package authn

import (
	"testing"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func TestMain(m *testing.M) {
	if err := log.Init("DEBUG"); err != nil {
		panic(err)
	}
	m.Run()
}

func validClaims(orgHandle string) map[string]interface{} {
	return map[string]interface{}{
		constants.OrgHandleClaim: orgHandle,
		constants.ExpiryClaim:    float64(time.Now().Add(time.Hour).Unix()),
		constants.AudienceClaim:  expectedAudience,
	}
}

func TestValidateClaims_Wso2is_RequiresMatchingOrgHandle(t *testing.T) {
	claims := validClaims("org-a")

	if !validateClaims("org-a", claims, constants.IdentityProviderWSO2IS) {
		t.Error("expected claims with a matching org_handle to validate for wso2is")
	}
	if validateClaims("org-b", claims, constants.IdentityProviderWSO2IS) {
		t.Error("expected claims with a mismatched org_handle to fail validation for wso2is")
	}
}

func TestValidateClaims_Thunder_SkipsOrgHandleCheck(t *testing.T) {
	// Thunder tokens carry no org_handle claim at all - the check must be
	// skipped entirely, not merely tolerant of a missing claim.
	claims := map[string]interface{}{
		constants.ExpiryClaim:   float64(time.Now().Add(time.Hour).Unix()),
		constants.AudienceClaim: expectedAudience,
	}

	if !validateClaims("any-org", claims, constants.IdentityProviderThunder) {
		t.Error("expected claims without an org_handle claim to still validate for thunder")
	}
}

func TestValidateClaims_StillEnforcesExpiryAndAudienceForThunder(t *testing.T) {
	expired := map[string]interface{}{
		constants.ExpiryClaim:   float64(time.Now().Add(-time.Hour).Unix()),
		constants.AudienceClaim: expectedAudience,
	}
	if validateClaims("any-org", expired, constants.IdentityProviderThunder) {
		t.Error("expected an expired token to fail validation even for thunder")
	}

	wrongAudience := map[string]interface{}{
		constants.ExpiryClaim:   float64(time.Now().Add(time.Hour).Unix()),
		constants.AudienceClaim: "some-other-audience",
	}
	if validateClaims("any-org", wrongAudience, constants.IdentityProviderThunder) {
		t.Error("expected a mismatched audience to fail validation even for thunder")
	}
}
