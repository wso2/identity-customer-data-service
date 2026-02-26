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

type ProfileSchemaSync struct {
	OrgId string `json:"orgHandle" bson:"orgHandle"`
	Event string `json:"event" bson:"event"`
}

type ProfileSchemaAddAttribute struct {
	AttributeId           string           `json:"attribute_id" bson:"attribute_id"`
	AttributeName         string           `json:"attribute_name" bson:"attribute_name" binding:"required"`
	DisplayName           string           `json:"display_name,omitempty" bson:"display_name,omitempty"`
	ValueType             string           `json:"value_type" bson:"value_type" binding:"required"`
	MergeStrategy         string           `json:"merge_strategy" bson:"merge_strategy" binding:"required"`
	Mutability            string           `json:"mutability" bson:"mutability"`
	ApplicationIdentifier string           `json:"application_identifier,omitempty" bson:"application_identifier,omitempty"`
	MultiValued           bool             `json:"multi_valued,omitempty" bson:"multi_valued,omitempty"`         // Means the data type is an array of chosen data type
	CanonicalValues       []CanonicalValue `json:"canonical_values,omitempty" bson:"canonical_values,omitempty"` // String of options for the attribute
	SubAttributes         []SubAttribute   `json:"sub_attributes,omitempty" bson:"sub_attributes,omitempty"`     // If the datatype is object
}
