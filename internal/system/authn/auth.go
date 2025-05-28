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

package authn

import (
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/identity-customer-data-service/internal/event_stream_ids/store"
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/system/cache"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AuthService struct {
	store store.EventStreamIdStoreInterface
}

var (
	tokenCache       = cache.NewCache(15 * time.Minute)
	expectedAudience = "iam-cds"
)

// ValidateAuthenticationAndReturnClaims validates Authorization: Bearer token from the HTTP request
func ValidateAuthenticationAndReturnClaims(token string) (map[string]interface{}, error) {
	// Try cache
	if cached, found := tokenCache.Get(token); found {
		if claims, ok := cached.(map[string]interface{}); ok && validateClaims(claims) {
			return claims, nil
		}
	}

	var claims map[string]interface{}
	var err error

	// Detect if token is JWT or opaque (very naive check: JWT has two dots)
	if strings.Count(token, ".") == 2 {
		claims, err = ParseJWTClaims(token)
		if err != nil {
			return claims, unauthorizedError()
		}
	} else {
		claims, err = IntrospectOpaqueToken(token)
		if err != nil {
			return claims, unauthorizedError()
		}
	}

	if !validateClaims(claims) {
		return claims, unauthorizedError()
	}

	tokenCache.Set(token, claims)
	return claims, nil
}

// ParseJWTClaims parses claims from a JWT without verifying the signature
func ParseJWTClaims(tokenString string) (map[string]interface{}, error) {

	logger := log.GetLogger()
	claims := jwt.MapClaims{}
	_, _, err := new(jwt.Parser).ParseUnverified(tokenString, claims)
	if err != nil {
		errMsg := "Error occurred when parsing claims from JWT token."
		logger.Debug(errMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.PARSING_ERROR.Code,
			Message:     errors2.PARSING_ERROR.Message,
			Description: errMsg,
		}, err)
		return nil, serverError
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
	authSvc := &AuthService{
		store: &store.EventStreamIdStore{},
	}
	return authSvc.validateEventStreamId(token, orgID, appID)
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

func (s *AuthService) validateEventStreamId(eventStreamId, orgID, appID string) (valid bool, error error) {
	if eventStreamId == "" || appID == "" || orgID == "" {
		return false, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "missing_fields",
			Message:     "Missing required fields",
			Description: "event_stream_id, application_id, or org_id is missing",
		}, http.StatusBadRequest)
	}

	dbKey, err := s.store.GetEventStreamId(eventStreamId)
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

// IntrospectOpaqueToken introspects an opaque token t
func IntrospectOpaqueToken(token string) (map[string]interface{}, error) {

	form := url.Values{}
	form.Set("token", token)

	runtimeConfig := config.GetCDSRuntime().Config
	authServerConfig := runtimeConfig.AuthServer

	req, err := http.NewRequest("POST", authServerConfig.IntrospectionEndPoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(authServerConfig.AdminUsername, authServerConfig.AdminPassword)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to introspect token."
		logger.Debug(errorMsg, log.Error(err))
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.INTROSPECTION_FAILED.Code,
			Message:     errors2.INTROSPECTION_FAILED.Description,
			Description: errorMsg,
		}, http.StatusUnauthorized)
		return nil, clientError
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, unauthorizedError()
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}
