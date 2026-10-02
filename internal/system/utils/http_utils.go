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

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors" // Standard Go errors package
	"fmt"
	"net/http"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	customerrors "github.com/wso2/identity-customer-data-service/internal/system/errors" // Alias for the custom errors
	error2 "github.com/wso2/identity-customer-data-service/internal/system/errors"       // Importing custom error types
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

// HandleError sends an HTTP error response based on the provided error
func HandleError(w http.ResponseWriter, err error) {
	var clientError *customerrors.ClientError
	w.Header().Set("Content-Type", "application/json")
	if ok := errors.As(err, &clientError); ok {
		w.WriteHeader(clientError.StatusCode)
		_ = json.NewEncoder(w).Encode(struct {
			Code        string `json:"code"`
			Message     string `json:"message"`
			Description string `json:"description"`
		}{
			Code:        clientError.ErrorMessage.Code,
			Message:     clientError.ErrorMessage.Message,
			Description: clientError.ErrorMessage.Description,
		})
		return
	}

	var serverError *customerrors.ServerError
	if ok := errors.As(err, &serverError); ok {
		logger := log.GetLogger()
		logger.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Internal server error",
		})
		return
	}
}

func ExtractPathPrefixFromContext(r *http.Request) string {
	if prefix, ok := r.Context().Value(constants.PathPrefixContextKey).(string); ok && prefix != "" {
		return prefix
	}
	return "/t/"
}

func ExtractOrgHandleFromPath(r *http.Request) string {
	tenant := r.Context().Value(constants.TenantContextKey).(string)
	if tenant == "" {
		// If tenant is not found in context, fallback to default tenant
		tenant = "carbon.super"
	}
	return tenant
}

func StripTenantPrefix(path string) string {
	parts := strings.SplitN(path, "/", 4)
	if len(parts) < 4 {
		return "/"
	}
	return "/" + parts[3]
}

func WriteErrorResponse(w http.ResponseWriter, err *error2.ClientError) {

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)

	_ = json.NewEncoder(w).Encode(err.ErrorMessage)
}

func WriteBadRequestErrorResponse(w http.ResponseWriter, err *error2.ClientError) {

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	_ = json.NewEncoder(w).Encode("Invalid erquest format")
}

// mountPrefixDispatcher is the shared implementation for tenant-style prefix dispatchers.
// It extracts the identifier segment immediately after prefix, stores it in context under
// TenantContextKey, rewrites the request path to the remaining suffix, and calls handlerFunc.
func mountPrefixDispatcher(mux *http.ServeMux, prefix string, handlerFunc http.HandlerFunc) {
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")

		if !strings.HasPrefix(path, prefix) {
			http.NotFound(w, r)
			return
		}

		parts := strings.SplitN(path[len(prefix):], "/", 2)
		if len(parts) != 2 {
			http.Error(w, "Invalid path format", http.StatusBadRequest)
			return
		}

		orgHandle := parts[0]
		remainingPath := "/" + parts[1]

		ctx := context.WithValue(r.Context(), constants.TenantContextKey, orgHandle)
		ctx = context.WithValue(ctx, constants.PathPrefixContextKey, prefix)
		r = r.WithContext(ctx)
		r.URL.Path = remainingPath

		handlerFunc(w, r)
	})
}

// MountTenantDispatcher mounts a dispatcher under /t/{tenant}/... and forwards requests to handlerFunc
// with the tenant added to the context. It preserves the remaining path (e.g., /cds/api/v1/...).
func MountTenantDispatcher(mux *http.ServeMux, handlerFunc http.HandlerFunc) {
	mountPrefixDispatcher(mux, "/t/", handlerFunc)
}

// MountOrgDispatcher mounts a dispatcher under /o/{org-uuid}/... and forwards requests to handlerFunc
// with the org UUID stored in context under TenantContextKey, preserving the remaining path.
func MountOrgDispatcher(mux *http.ServeMux, handlerFunc http.HandlerFunc) {
	mountPrefixDispatcher(mux, "/o/", handlerFunc)
}

// RespondJSON sends a JSON response with the given status code and payload
func RespondJSON(w http.ResponseWriter, status int, payload any, resource string) {

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(payload); err != nil {
		serverError := error2.NewServerError(error2.ErrorMessage{
			Code:        error2.ENCODE_ERROR.Code,
			Message:     error2.ENCODE_ERROR.Message,
			Description: fmt.Sprintf("Failed to encode %s response", resource),
		}, err)
		HandleError(w, serverError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
