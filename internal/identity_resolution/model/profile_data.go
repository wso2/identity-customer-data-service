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

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

type BlockingKey struct {
	AttributeName string
	KeyValue      string
}

func DetermineProfileType(attrs map[string]interface{}) string {
	if uid, ok := attrs["user_id"]; ok && uid != nil && uid != "" {
		return constants.ProfileTypePermanent
	}
	return constants.ProfileTypeTemp
}

func DetermineMode(inputType, candidateType string, smartResolutionEnabled bool) string {
	if !smartResolutionEnabled {
		return constants.UnificationModeStrict
	}
	if inputType == constants.ProfileTypeTemp && candidateType == constants.ProfileTypeTemp {
		return constants.UnificationModeStrict
	}
	return constants.UnificationModeSmart
}

type ProfileData struct {
	ProfileID          string
	UserID             string
	OrgHandle          string
	ReferenceProfileID string // non-empty if this profile is a child (merged into another)
	Attributes         map[string]interface{}
}

func (p *ProfileData) GetAttribute(name string) string {
	if p.Attributes == nil {
		return ""
	}
	if v, ok := p.Attributes[name]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func (p *ProfileData) GetProfileType() string {
	if p.UserID != "" {
		return constants.ProfileTypePermanent
	}
	return constants.ProfileTypeTemp
}

// IsChild returns true if this profile has been merged into another profile.
func (p *ProfileData) IsChild() bool {
	return p.ReferenceProfileID != ""
}
