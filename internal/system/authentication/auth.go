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
	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/store"
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/system/cache"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strings"
	"time"
)

var (
	tokenCache       = cache.NewCache(15 * time.Minute)
	expectedAudience = "iam-cds"
)

// ValidateAuthentication validates Authorization: Bearer token from the HTTP request
func ValidateAuthentication(r *http.Request) (bool, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return false, err
	}
	return validateToken(token)
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", unauthorizedError()
	}
	return strings.TrimPrefix(authHeader, "Bearer "), nil
}

func validateToken(token string) (bool, error) {
	//  Try cache
	if cached, found := tokenCache.Get(token); found {
		if claims, ok := cached.(map[string]interface{}); ok {
			if valid := validateClaims(claims); valid {
				return true, nil
			}
		}
	}

	//  todo: For now assume JWT — parse claims. For opaque need to call introspection endpoint
	claims, err := ParseJWTClaims(token)
	if err != nil {
		return false, unauthorizedError()
	}

	//  Validate claims
	if !validateClaims(claims) {
		return false, unauthorizedError()
	}

	//  Store in cache
	tokenCache.Set(token, claims)

	return true, nil
}

// parseJWTClaims parses claims from a JWT without verifying the signature
func ParseJWTClaims(tokenString string) (map[string]interface{}, error) {

	logger := log.GetLogger()
	logger.Debug("Parsing JWT claims?")
	claims := jwt.MapClaims{}
	_, _, err := new(jwt.Parser).ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// validateClaims ensures the token has `active: true` and the expected audience
func validateClaims(claims map[string]interface{}) bool {

	expRaw, ok := claims["exp"]
	if !ok {
		return false
	}
	expFloat, ok := expRaw.(float64)
	if !ok {
		return false
	}
	expUnix := int64(expFloat)
	currentTime := time.Now().Unix()
	if expUnix < currentTime {
		return false
	}

	audRaw, ok := claims["aud"]
	if !ok {
		return false
	}

	var audList []string
	switch aud := audRaw.(type) {
	case []interface{}:
		for _, a := range aud {
			if s, ok := a.(string); ok {
				audList = append(audList, s)
			}
		}
	case string:
		audList = append(audList, aud)
	}

	for _, aud := range audList {
		if aud == expectedAudience {
			return true
		}
	}
	return false
}

// GetCachedClaims returns claims from cache if available
func GetCachedClaims(token string) (map[string]interface{}, bool) {
	cached, found := tokenCache.Get(token)
	if !found {
		return nil, false
	}
	claims, ok := cached.(map[string]interface{})
	return claims, ok
}

func unauthorizedError() error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:        errors2.UN_AUTHORIZED.Code,
		Message:     errors2.UN_AUTHORIZED.Message,
		Description: errors2.UN_AUTHORIZED.Description,
	}, http.StatusUnauthorized)
}

// ValidateAuthenticationForEvent validates Authorization: ApiKey header from the HTTP request
func ValidateAuthenticationForEvent(r *http.Request, event model.Event) (valid bool, error error) {
	token, err := extractAPIKey(r)
	if err != nil {
		return false, err
	}
	orgID, appID := fetchOrgAndApp(event)
	return validateEventStreamId(token, orgID, appID)
}

func extractAPIKey(r *http.Request) (string, error) {

	logger := log.GetLogger()
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UN_AUTHORIZED.Code,
			Message:     errors2.UN_AUTHORIZED.Message,
			Description: errors2.UN_AUTHORIZED.Description,
		}, http.StatusUnauthorized)
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "eventStreamId" {

		logger.Warn("Invalid Event stream id format")
		return "", errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.UN_AUTHORIZED.Code,
			Message:     errors2.UN_AUTHORIZED.Message,
			Description: errors2.UN_AUTHORIZED.Description,
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
		return false, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "missing_fields",
			Message:     "Missing required fields",
			Description: "event_stream_id, application_id, or org_id is missing",
		}, http.StatusBadRequest)
	}

	dbKey, err := store.GetEventStreamId(eventStreamId)
	if err != nil || dbKey == nil {
		return false, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "invalid_api_key",
			Message:     "API key not found",
			Description: "Provided API key is not valid",
		}, http.StatusUnauthorized)
	}

	if dbKey.AppID != appID {
		return false, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "mismatch_app_id",
			Message:     "App ID does not match",
			Description: "API key does not belong to this application",
		}, http.StatusUnauthorized)
	}

	if dbKey.OrgID != orgID {
		if !(orgID == "carbon.super" && dbKey.OrgID == "-1234") && !(dbKey.OrgID == "carbon.super" && orgID == "-1234") {
			return false, errors2.NewClientError(errors2.ErrorMessage{
				Code:        "mismatch_org_id",
				Message:     "Org ID does not match",
				Description: "API key does not belong to this organization",
			}, http.StatusUnauthorized)
		}
	}

	if dbKey.State != "active" {
		return false, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "revoked",
			Message:     "API key is not active",
			Description: "API key is revoked or inactive",
		}, http.StatusUnauthorized)
	}

	now := time.Now().UTC().Unix()
	if dbKey.ExpiresAt < now {
		return false, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "expired",
			Message:     "API key has expired",
			Description: "API key expiration time has passed",
		}, http.StatusUnauthorized)
	}

	return true, nil
}
