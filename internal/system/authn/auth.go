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
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/identity-customer-data-service/internal/system/client"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

var (
	expectedAudience = "iam-cds"
)

// ValidateAuthenticationAndReturnClaims validates Authorization: Bearer token from the HTTP request
func ValidateAuthenticationAndReturnClaims(token, orgHandle string) (map[string]interface{}, error) {

	var claims map[string]interface{}
	var err error
	logger := log.GetLogger()

	// Detect if token is JWT or opaque
	if strings.Count(token, ".") == 2 {
		claims, err = ParseJWTClaims(token)
		if err != nil {
			return claims, unauthorizedError()
		}
		cfg := config.GetCDSRuntime().Config
		identityClient := client.NewIdentityClient(cfg)
		introspectionClaims, err := identityClient.IntrospectToken(token, orgHandle)
		if err != nil {
			return claims, unauthorizedError()
		}
		active, ok := introspectionClaims["active"].(bool)
		if !ok || !active {
			logger.Debug("JWT token is not active according to introspection.")
			return nil, unauthorizedError()
		}
	} else {
		logger.Debug("Expecting a JWT token but received an opaque token.")
		return claims, unauthorizedError()
	}

	if !validateClaims(orgHandle, claims) {
		return claims, unauthorizedError()
	}

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

// validateClaims ensures the token has `active: true` and the expected audience and org_handle
func validateClaims(orgHandle string, claims map[string]interface{}) bool {

	logger := log.GetLogger()
	orgHandleInClaimRaw, ok := claims["org_handle"]
	if !ok || orgHandleInClaimRaw != orgHandle {
		logger.Debug("Token does not have the expected org_handle claim.")
		return false
	}
	orgHandleInClaim, ok := orgHandleInClaimRaw.(string)
	if !ok || orgHandleInClaim != orgHandle {
		logger.Debug("Token org_handle claim is not valid.")
		return false
	}

	expRaw, ok := claims["exp"]
	if !ok {
		logger.Debug("Token does not have an expiration time.")
		return false
	}
	expFloat, ok := expRaw.(float64)
	if !ok {
		logger.Debug("Token does not have a valid expiration time.", log.Any("exp", expRaw))
		return false
	}
	expUnix := int64(expFloat)
	currentTime := time.Now().Unix()
	if expUnix < currentTime {
		logger.Debug("Token has expired.", log.String("exp", time.Unix(expUnix, 0).String()))
		return false
	}

	audRaw, ok := claims["aud"]
	if !ok {
		logger.Debug("Token does not have an audience claim.")
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
	logger.Debug("Token audience does not match expected audience.")
	return false
}

func unauthorizedError() error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:        errors2.UN_AUTHORIZED.Code,
		Message:     errors2.UN_AUTHORIZED.Message,
		Description: errors2.UN_AUTHORIZED.Description,
	}, http.StatusUnauthorized)
}
