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
	"context"
	"encoding/json"
	"errors" // Standard Go errors package
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	customerrors "github.com/wso2/identity-customer-data-service/internal/system/errors" // Alias for the custom errors
	error2 "github.com/wso2/identity-customer-data-service/internal/system/errors"       // Importing custom error types
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"net/http"
	"strings"
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

func ExtractTenantIdFromPath(r *http.Request) string {
	//path := r.URL.Path
	//parts := strings.Split(path, "/")
	//for i := 0; i < len(parts)-1; i++ {
	//	if parts[i] == "t" {
	//		return parts[i+1]
	//	}
	//}
	//return "carbon.super" // fallback default
	tenant := r.Context().Value(constants.TenantContextKey).(string)
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

// Rewrite `/api/v1/...` to `/t/carbon.super/api/v1/...`
func RewriteToDefaultTenant(apiBasePath string, mux *http.ServeMux, defaultTenant string) {
	mux.HandleFunc(apiBasePath+"/", func(w http.ResponseWriter, r *http.Request) {
		newPath := "/t/" + defaultTenant + r.URL.Path
		http.Redirect(w, r, newPath, http.StatusTemporaryRedirect)
	})
}

func MountTenantDispatcher(mux *http.ServeMux, apiBasePath string, handlerFunc http.HandlerFunc) {
	mux.HandleFunc("/t/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")

		if !strings.HasPrefix(path, "/t/") {
			http.NotFound(w, r)
			return
		}

		// Split: /t/{tenant}/api/v1/...
		parts := strings.SplitN(path[len("/t/"):], "/", 2)
		if len(parts) != 2 {
			http.Error(w, "Invalid tenant path format", http.StatusBadRequest)
			return
		}

		tenantID := parts[0]
		remainingPath := "/" + parts[1]

		// Ensure it starts with apiBasePath
		if !strings.HasPrefix(remainingPath, apiBasePath) {
			http.Error(w, "Path must start with "+apiBasePath, http.StatusNotFound)
			return
		}

		// Strip /api/v1 to route to /profiles, etc.
		relativePath := strings.TrimPrefix(remainingPath, apiBasePath)

		// Add tenant to request context
		ctx := context.WithValue(r.Context(), constants.TenantContextKey, tenantID)
		r = r.WithContext(ctx)
		r.URL.Path = relativePath

		// Optional: debug log
		// fmt.Printf("[Dispatcher] tenant=%s path=%s\n", tenantID, relativePath)

		handlerFunc(w, r)
	})
}
