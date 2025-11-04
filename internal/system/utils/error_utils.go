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
	"github.com/pkg/errors"
	"io"
	"strings"
)

func HandleDecodeError(err error, resourceName string) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, io.EOF):
		return fmt.Sprintf("Request body for %s is empty.", resourceName)

	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// Extract offending field name from the error
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Sprintf("Unknown field %s in %s request body.", field, resourceName)

	case errors.As(err, &json.UnmarshalTypeError{}):
		var ute *json.UnmarshalTypeError
		errors.As(err, &ute)
		if ute != nil {
			return fmt.Sprintf("Invalid type for field '%s'. Expected %s in %s request body.",
				ute.Field, ute.Type, resourceName)
		}
		return fmt.Sprintf("Invalid type in %s request body.", resourceName)

	case errors.As(err, &json.SyntaxError{}):
		var se *json.SyntaxError
		errors.As(err, &se)
		if se != nil {
			return fmt.Sprintf("Malformed JSON at position %d in %s request body.", se.Offset, resourceName)
		}
		return fmt.Sprintf("Malformed JSON in %s request body.", resourceName)

	default:
		return fmt.Sprintf("Invalid JSON payload for %s.", resourceName)
	}
}
