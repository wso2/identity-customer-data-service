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
	"encoding/json"
	"net/http"

	"github.com/wso2/identity-customer-data-service/internal/sharing/model"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

// DecodePolicyRequest reads the body of a POST or a PUT on a policy.
func DecodePolicyRequest(w http.ResponseWriter, r *http.Request) (model.PolicyRequest, bool) {

	var req model.PolicyRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.SHARE_BAD_REQUEST.Code,
			Message:     errors2.SHARE_BAD_REQUEST.Message,
			Description: utils.HandleDecodeError(err, "policy request"),
		}, http.StatusBadRequest))
		return req, false
	}
	return req, true
}
