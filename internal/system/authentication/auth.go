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

package authentication

import (
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/store"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
)

// ValidateAuthentication validates Authorization: ApiKey header from the HTTP request
func ValidateAuthentication(r *http.Request, event model.Event) (valid bool, error error) {
	token, err := extractAPIKey(r)
	if err != nil {
		return false, err
	}
	orgID, appID := fetchOrgAndApp(event)
	return validateEventStreamId(token, orgID, appID)
}

func extractAPIKey(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedRequest.Code,
			Message:     errors.ErrUnAuthorizedRequest.Message,
			Description: errors.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "eventStreamId" {
		log.Print("Invalid API key format")
		return "", errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedRequest.Code,
			Message:     errors.ErrUnAuthorizedRequest.Message,
			Description: errors.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
	}
	return parts[1], nil
}

func fetchOrgAndApp(event model.Event) (orgID, appID string) {
	// Extract values safely
	orgID = event.OrgId
	appID = event.AppId

	// Normalize default tenant
	if orgID == "carbon.super" {
		orgID = "-1234"
	}

	return orgID, appID
}

func validateEventStreamId(eventStreamId, orgID, appID string) (valid bool, error error) {
	if eventStreamId == "" || appID == "" || orgID == "" {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "missing_fields",
			Message:     "Missing required fields",
			Description: "event_stream_id, application_id, or org_id is missing",
		}, http.StatusBadRequest)
	}

	dbKey, err := store.GetEventStreamId(eventStreamId)
	if err != nil || dbKey == nil {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "invalid_api_key",
			Message:     "API key not found",
			Description: "Provided API key is not valid",
		}, http.StatusUnauthorized)
	}

	if dbKey.AppID != appID {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "mismatch_app_id",
			Message:     "App ID does not match",
			Description: "API key does not belong to this application",
		}, http.StatusUnauthorized)
	}

	if dbKey.OrgID != orgID {
		if (orgID == "carbon.super" && dbKey.OrgID == "-1234") ||
			(dbKey.OrgID == "carbon.super" && orgID == "-1234") {
			orgID = "carbon.super"
		} else {
			return false, errors.NewClientError(errors.ErrorMessage{
				Code:        "mismatch_org_id",
				Message:     "Org ID does not match",
				Description: "API key does not belong to this organization",
			}, http.StatusUnauthorized)
		}
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "mismatch_org_id",
			Message:     "Org ID does not match",
			Description: "API key does not belong to this organization",
		}, http.StatusUnauthorized)
	}

	if dbKey.State != "active" {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "revoked",
			Message:     "API key is not active",
			Description: "API key is revoked or inactive",
		}, http.StatusUnauthorized)
	}

	now := time.Now().UTC().Unix()
	if dbKey.ExpiresAt < now {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "expired",
			Message:     "API key has expired",
			Description: "API key expiration time has passed",
		}, http.StatusUnauthorized)
	}

	return true, nil
}
