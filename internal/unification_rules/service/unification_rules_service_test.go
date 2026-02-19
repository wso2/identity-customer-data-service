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
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
)

func TestMain(m *testing.M) {
	_ = log.Init("ERROR")
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// AddUnificationRule – early-return validation (no DB required)
// ---------------------------------------------------------------------------

func TestAddUnificationRule_UserId_Rejected(t *testing.T) {
	svc := &UnificationRuleService{}
	rule := model.UnificationRule{
		PropertyName: "user_id",
		OrgHandle:    "org1",
		RuleName:     "test-rule",
		Priority:     1,
	}
	err := svc.AddUnificationRule(rule, "org1")
	require.Error(t, err)

	clientErr, ok := err.(*errors.ClientError)
	require.True(t, ok, "expected a ClientError")
	assert.Equal(t, http.StatusConflict, clientErr.StatusCode)
}

func TestAddUnificationRule_ApplicationDataPrefix_Rejected(t *testing.T) {
	svc := &UnificationRuleService{}
	rule := model.UnificationRule{
		PropertyName: "application_data.some_field",
		OrgHandle:    "org1",
		RuleName:     "test-rule",
		Priority:     2,
	}
	err := svc.AddUnificationRule(rule, "org1")
	require.Error(t, err)

	clientErr, ok := err.(*errors.ClientError)
	require.True(t, ok, "expected a ClientError")
	assert.Equal(t, http.StatusBadRequest, clientErr.StatusCode)
}

// ---------------------------------------------------------------------------
// PatchUnificationRule – user_id guard (no DB required)
// ---------------------------------------------------------------------------

func TestPatchUnificationRule_UserId_Rejected(t *testing.T) {
	svc := &UnificationRuleService{}
	updated := model.UnificationRule{
		PropertyName: "user_id",
		Priority:     1,
		IsActive:     true,
	}
	err := svc.PatchUnificationRule("rule-123", "org1", updated)
	require.Error(t, err)

	clientErr, ok := err.(*errors.ClientError)
	require.True(t, ok, "expected a ClientError")
	assert.Equal(t, http.StatusBadRequest, clientErr.StatusCode)
}
