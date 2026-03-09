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
	"fmt"
	"strings"
)

func BuildProfileLocation(orgId, profileId string) string {
	return fmt.Sprintf("%s/cds/api/v1/profiles/%s", orgId, profileId)
}

// ResolveDisplayName takes an attribute name (potentially in dot notation) and converts it to a human-readable display name.
func ResolveDisplayName(attributeName string) string {
	parts := strings.Split(attributeName, ".")
	if len(parts) == 0 {
		return ""
	}

	// Take leaf attribute for display name
	leaf := parts[len(parts)-1]

	// Title case each word
	words := strings.Fields(leaf)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}

	return strings.Join(words, " ")
}
