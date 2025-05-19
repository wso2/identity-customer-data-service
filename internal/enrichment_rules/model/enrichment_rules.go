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

type ProfileEnrichmentRule struct {
	RuleId            string      `json:"rule_id,omitempty" bson:"rule_id,omitempty"`
	PropertyName      string      `json:"property_name" bson:"property_name"`
	Description       string      `json:"description,omitempty" bson:"description,omitempty"`
	Value             interface{} `json:"value,omitempty" bson:"value,omitempty"` // required if computation == static
	ValueType         string      `json:"value_type,omitempty" bson:"value_type,omitempty"`
	ComputationMethod string      `json:"computation_method,omitempty" bson:"computation_method,omitempty"` // if trait_type == computed
	SourceField       string      `json:"source_field,omitempty" bson:"source_field,omitempty"`             // For concat
	TimeRange         int64       `json:"time_range,omitempty" bson:"time_range,omitempty"`                 // last x seconds - required for count, sum, avg
	MergeStrategy     string      `json:"merge_strategy" bson:"merge_strategy"`                             // overwrite, combine, ignore
	Trigger           RuleTrigger `json:"trigger" bson:"trigger"`
	CreatedAt         int64       `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt         int64       `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

type RuleTrigger struct {
	EventType  string          `json:"event_type" bson:"event_type"`
	EventName  string          `json:"event_name" bson:"event_name"`
	Conditions []RuleCondition `json:"conditions" bson:"conditions"`
}

type RuleCondition struct {
	Field    string `json:"field" bson:"field"`
	Operator string `json:"operator" bson:"operator"`
	Value    string `json:"value" bson:"value"`
}
