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

package model

type ProfileHierarchy struct {
	ParentProfileID string         `json:"parent_profile_id,omitempty" bson:"parent_profile_id,omitempty"`
	IsParent        bool           `json:"is_parent,omitempty" bson:"is_parent,omitempty"`
	ListProfile     bool           `json:"list_profile,omitempty" bson:"list_profile,omitempty"`
	ChildProfiles   []ChildProfile `json:"child_profile_ids,omitempty" bson:"child_profile_ids,omitempty"`
}

type ChildProfile struct {
	ChildProfileId string `json:"child_profile_id,omitempty" bson:"child_profile_id,omitempty"`
	RuleName       string `json:"rule_name,omitempty" bson:"rule_name,omitempty"`
}

type Profile struct {
	ProfileId          string                 `json:"profile_id" bson:"profile_id"`
	OriginCountry      string                 `json:"origin_country" bson:"origin_country"`
	IdentityAttributes map[string]interface{} `json:"identity_attributes,omitempty" bson:"identity_attributes,omitempty"`
	Traits             map[string]interface{} `json:"traits,omitempty" bson:"traits,omitempty"`
	ApplicationData    []ApplicationData      `json:"application_data,omitempty" bson:"application_data,omitempty"`
	ProfileHierarchy   *ProfileHierarchy      `json:"profile_hierarchy,omitempty" bson:"profile_hierarchy,omitempty"`
}
