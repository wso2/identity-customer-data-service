/*
 * Copyright (c) 2025-2026, WSO2 LLC. (http://www.wso2.com).
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

package security

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/system/authn"
	"github.com/wso2/identity-customer-data-service/internal/system/authz"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	cdscontext "github.com/wso2/identity-customer-data-service/internal/system/context"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

// AuthnWithAdminCredentials performs authentication using admin credentials from the request.
func AuthnWithAdminCredentials(r *http.Request) error {

	authHeader := r.Header.Get("Authorization")
	logger := log.GetLogger()
	traceID := cdscontext.GetTraceID(r.Context())

	if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
		logger.Debug("Missing or invalid Authorization header")
		// Audit failed authentication
		logger.Audit(log.AuditEvent{
			InitiatorID:   "unknown",
			InitiatorType: log.InitiatorTypeAdmin,
			TargetID:      "system",
			TargetType:    "authentication",
			ActionID:      log.ActionAuthenticationFailure,
			TraceID:       traceID,
			Data:          map[string]string{"reason": "missing_or_invalid_header"},
		})
		return errors.NewClientErrorWithTraceID(errors.ErrorMessage{
			Code:        errors.UN_AUTHORIZED.Code,
			Message:     errors.UN_AUTHORIZED.Message,
			Description: "Missing or invalid Authorization header",
		}, http.StatusUnauthorized, traceID)
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Basic "))

	isValidAdmin, err := validateAdminCredentials(token)
	if err != nil || !isValidAdmin {
		logger.Debug("Invalid admin credentials")
		// Audit failed authentication
		logger.Audit(log.AuditEvent{
			InitiatorID:   "unknown",
			InitiatorType: log.InitiatorTypeAdmin,
			TargetID:      "system",
			TargetType:    "authentication",
			ActionID:      log.ActionAuthenticationFailure,
			TraceID:       traceID,
			Data:          map[string]string{"reason": "invalid_credentials"},
		})
		return errors.NewClientErrorWithTraceID(errors.ErrorMessage{
			Code:        errors.UN_AUTHORIZED.Code,
			Message:     errors.UN_AUTHORIZED.Message,
			Description: "Missing or invalid Authorization header",
		}, http.StatusUnauthorized, traceID)
	}

	// Audit successful authentication
	logger.Audit(log.AuditEvent{
		InitiatorID:   "admin",
		InitiatorType: log.InitiatorTypeAdmin,
		TargetID:      "system",
		TargetType:    "authentication",
		ActionID:      log.ActionAuthenticationSuccess,
		TraceID:       traceID,
	})

	return nil
}

func validateAdminCredentials(token string) (bool, error) {

	authServerConfig := config.GetCDSRuntime().Config.AuthServer
	username := strings.TrimSpace(authServerConfig.AdminUsername)
	password := strings.TrimSpace(authServerConfig.AdminPassword)
	logger := log.GetLogger()
	if username == "" || password == "" || token == "" {
		logger.Debug("Admin credentials are not set properly in the configuration.")
		return false, nil
	}

	creds := username + ":" + password
	expected := base64.StdEncoding.EncodeToString([]byte(creds))

	if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1 {
		logger.Debug("Admin credentials validated successfully.")
		return true, nil
	}

	return false, nil
}

// AuthnAndAuthz performs authentication and authorization for the given HTTP request and operation.
func AuthnAndAuthz(r *http.Request, operation string) error {

	authHeader := r.Header.Get("Authorization")
	logger := log.GetLogger()
	traceID := cdscontext.GetTraceID(r.Context())

	if !strings.HasPrefix(authHeader, "Bearer ") || authHeader == "" {
		// Audit failed authentication
		logger.Audit(log.AuditEvent{
			InitiatorID:   "unknown",
			InitiatorType: log.InitiatorTypeUser,
			TargetID:      "system",
			TargetType:    "authentication",
			ActionID:      log.ActionAuthenticationFailure,
			TraceID:       traceID,
			Data:          map[string]string{"reason": "missing_or_invalid_bearer_token"},
		})
		clientError := errors.NewClientErrorWithTraceID(errors.ErrorMessage{
			Code:        errors.UN_AUTHORIZED.Code,
			Message:     errors.UN_AUTHORIZED.Message,
			Description: "Missing or invalid Authorization header",
		}, http.StatusUnauthorized, traceID)
		return clientError
	}

	orgHandle := utils.ExtractOrgHandleFromPath(r)

	token := strings.TrimPrefix(authHeader, "Bearer ")

	//  Validate token
	claims, err := authn.ValidateAuthenticationAndReturnClaims(token, orgHandle)
	if err != nil {
		// Audit failed authentication
		logger.Audit(log.AuditEvent{
			InitiatorID:   "unknown",
			InitiatorType: log.InitiatorTypeUser,
			TargetID:      "system",
			TargetType:    "authentication",
			ActionID:      log.ActionAuthenticationFailure,
			TraceID:       traceID,
			Data:          map[string]string{"reason": "token_validation_failed"},
		})
		clientError := errors.NewClientErrorWithTraceID(errors.ErrorMessage{
			Code:        errors.UN_AUTHORIZED.Code,
			Message:     errors.UN_AUTHORIZED.Message,
			Description: "Missing or invalid Authorization header",
		}, http.StatusUnauthorized, traceID)
		return clientError
	}

	// Extract user ID from claims for audit logging
	userID := "unknown"
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		userID = sub
	}

	//  Validate authorization
	scope, ok := claims["scope"]
	if !ok || scope == nil {
		// Audit authorization failure
		logger.Audit(log.AuditEvent{
			InitiatorID:   userID,
			InitiatorType: log.InitiatorTypeUser,
			TargetID:      "system",
			TargetType:    "authorization",
			ActionID:      log.ActionAuthenticationFailure,
			TraceID:       traceID,
			Data:          map[string]string{"reason": "missing_scope"},
		})
		clientError := errors.NewClientErrorWithTraceID(errors.ErrorMessage{
			Code:        errors.FORBIDDEN.Code,
			Message:     errors.FORBIDDEN.Message,
			Description: errors.FORBIDDEN.Description,
		}, http.StatusForbidden, traceID)
		return clientError
	}

	if !authz.ValidatePermission(scope.(string), operation) {
		// Audit authorization failure
		logger.Audit(log.AuditEvent{
			InitiatorID:   userID,
			InitiatorType: log.InitiatorTypeUser,
			TargetID:      "system",
			TargetType:    "authorization",
			ActionID:      log.ActionAuthenticationFailure,
			TraceID:       traceID,
			Data:          map[string]string{"reason": "insufficient_permissions", "operation": operation},
		})
		clientError := errors.NewClientErrorWithTraceID(errors.ErrorMessage{
			Code:        errors.FORBIDDEN.Code,
			Message:     errors.FORBIDDEN.Message,
			Description: "Do not have permission to perform this operation",
		}, http.StatusForbidden, traceID)
		return clientError
	}

	// Audit successful authentication
	logger.Audit(log.AuditEvent{
		InitiatorID:   userID,
		InitiatorType: log.InitiatorTypeUser,
		TargetID:      "system",
		TargetType:    "authentication",
		ActionID:      log.ActionAuthenticationSuccess,
		TraceID:       traceID,
		Data:          map[string]string{"operation": operation},
	})

	return nil
}
