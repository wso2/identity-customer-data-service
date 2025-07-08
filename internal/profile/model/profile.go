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

type ProfileStatus struct {
	ReferenceProfileId string      `json:"reference_profile_id,omitempty" bson:"reference_profile_id,omitempty"`
	IsReferenceProfile bool        `json:"is_reference_profile,omitempty" bson:"is_reference_profile,omitempty"`
	IsWaitingOnAdmin   bool        `json:"is_waiting_on_admin,omitempty" bson:"is_waiting_on_admin,omitempty"`
	IsWaitingOnUser    bool        `json:"is_waiting_on_user,omitempty" bson:"is_waiting_on_user,omitempty"`
	DeleteProfile      bool        `json:"delete_profile,omitempty" bson:"delete_profile,omitempty"`
	ListProfile        bool        `json:"list_profile,omitempty" bson:"list_profile,omitempty"`
	ReferenceReason    string      `json:"reference_reason,omitempty" bson:"reference_reason,omitempty"`
	References         []Reference `json:"references,omitempty" bson:"references,omitempty"`
}

type Reference struct {
	ProfileId string `json:"profile_id,omitempty" bson:"profile_id,omitempty"`
	Reason    string `json:"reason,omitempty" bson:"rule_name,omitempty"`
}

type Profile struct {
	ProfileId          string                 `json:"profile_id" bson:"profile_id"`
	UserId             string                 `json:"user_id" bson:"user_id"`
	TenantId           string                 `json:"tenant_id" bson:"tenant_id"`
	CreatedAt          int64                  `json:"created_at" bson:"created_at"`
	UpdatedAt          int64                  `json:"updated_at" bson:"updated_at"`
	Location           string                 `json:"location" bson:"location"`
	IdentityAttributes map[string]interface{} `json:"identity_attributes,omitempty" bson:"identity_attributes,omitempty"`
	Traits             map[string]interface{} `json:"traits,omitempty" bson:"traits,omitempty"`
	ApplicationData    []ApplicationData      `json:"application_data,omitempty" bson:"application_data,omitempty"`
	ProfileStatus      *ProfileStatus         `json:"profile_status,omitempty" bson:"profile_status,omitempty"`
}
