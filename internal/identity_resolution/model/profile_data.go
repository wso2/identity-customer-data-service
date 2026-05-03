/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

package model

import (
	"fmt"
)

type BlockingKey struct {
	AttributeName string
	KeyValue      string
}

type ProfileData struct {
	ProfileID          string
	UserID             string
	OrgHandle          string
	ReferenceProfileID string // non-empty if this profile is a child (merged into another)
	Attributes         map[string]interface{}
}

// GetAllAttributeValues returns all non-empty string values for the given attribute name.
func (profile *ProfileData) GetAllAttributeValues(name string) []string {
	if profile.Attributes == nil {
		return nil
	}
	attrValue, ok := profile.Attributes[name]
	if !ok || attrValue == nil {
		return nil
	}
	switch typed := attrValue.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case []interface{}:
		var result []string
		for _, elem := range typed {
			if s, ok := elem.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	case []string:
		var result []string
		for _, s := range typed {
			if s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		s := fmt.Sprintf("%v", attrValue)
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

// IsChild returns true if this profile has been merged into another profile.
func (profile *ProfileData) IsChild() bool {
	return profile.ReferenceProfileID != ""
}
