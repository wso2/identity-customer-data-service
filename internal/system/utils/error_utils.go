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
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

// HandleDecodeError interprets JSON decoding errors and returns user-friendly messages.
func HandleDecodeError(err error, resourceName string) string {
	if err == nil {
		return ""
	}

	// Empty body
	if errors.Is(err, io.EOF) {
		return fmt.Sprintf("Request body for %s is empty.", resourceName)
	}

	// Unknown field (when using DisallowUnknownFields)
	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Sprintf("Unknown field %s in %s request body.", field, resourceName)
	}

	// Malformed JSON
	var se *json.SyntaxError
	if errors.As(err, &se) && se != nil {
		return fmt.Sprintf("Malformed JSON in %s request body.", resourceName)
	}

	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) && ute != nil {
		// Top-level mismatch (common: object sent, array expected)
		if ute.Field == "" && ute.Value == "object" {
			return fmt.Sprintf("Request body for %s must be a JSON array.", resourceName)
		}

		if ute.Field != "" {
			return fmt.Sprintf("Invalid type for field '%s' in %s request body.", ute.Field, resourceName)
		}

		// Field-level mismatch: mention the JSON field name only.
		return fmt.Sprintf("Invalid type for field '%s' in %s request body.", ute.Field, resourceName)
	}

	// Generic fallback
	return fmt.Sprintf("Invalid JSON payload for %s.", resourceName)
}
