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

type ProfileSchemaAttribute struct {
	OrgId         string `json:"org_id" bson:"org_id"`
	AttributeId   string `json:"attribute_id" bson:"attribute_id"`
	AttributeName string `json:"attribute_name" bson:"attribute_name" binding:"required"`
	ValueType     string `json:"value_type" bson:"value_type" binding:"required"`
	MergeStrategy string `json:"merge_strategy" bson:"merge_strategy" binding:"required"`
}
