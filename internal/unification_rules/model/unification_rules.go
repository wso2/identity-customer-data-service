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

// UnificationRule represents rules for merging user profiles
type UnificationRule struct {
	RuleId    string `json:"rule_id" bson:"rule_id" binding:"required"`
	RuleName  string `json:"rule_name" bson:"rule_name" binding:"required"`
	Property  string `json:"property_name" bson:"property_name" binding:"required"`
	Priority  int    `json:"priority" bson:"priority" binding:"required"`
	IsActive  bool   `json:"is_active" bson:"is_active" binding:"required"`
	CreatedAt int64  `json:"created_at" bson:"created_at"`
	UpdatedAt int64  `json:"updated_at" bson:"updated_at"`
}
