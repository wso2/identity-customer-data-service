/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package model

const (
	// ChangeTypeDeleted indicates the schema attribute was removed; all
	// profiles must have the corresponding key unset.
	ChangeTypeDeleted = "deleted"

	// ChangeTypeTypeChanged indicates the attribute's value_type changed.
	// Existing stored values are nullified because they may no longer be
	// valid under the new type.
	ChangeTypeTypeChanged = "type_changed"

	// ChangeTypeScalarToArray indicates multi_valued changed false → true.
	// Each stored scalar value is wrapped in a single-element array so
	// that no data is lost.
	ChangeTypeScalarToArray = "scalar_to_array"

	// ChangeTypeArrayToScalar indicates multi_valued changed true → false.
	// The first element of each stored array is kept; remaining elements
	// are discarded.
	ChangeTypeArrayToScalar = "array_to_scalar"

	// ChangeTypeComplexSubAttrRemoved indicates a sub-attribute was removed
	// from a complex attribute while the parent stays complex.  The sub-key's
	// value is moved from the nested object to a flat dotted key so no data
	// is lost.  KeyPath must have at least two elements: [parentKey, subKey].
	ChangeTypeComplexSubAttrRemoved = "complex_sub_attr_removed"

	// ChangeTypeComplexSubAttrAdded indicates a sub-attribute was added to a
	// complex attribute.  Any existing flat dotted key (written by a prior
	// ChangeTypeComplexSubAttrRemoved job) is moved back into the nested
	// object.  KeyPath must have at least two elements: [parentKey, subKey].
	ChangeTypeComplexSubAttrAdded = "complex_sub_attr_added"
)

// SchemaChangeJob carries the information needed to propagate a single
// schema attribute change to all affected profile rows.  Jobs are produced
// whenever a schema attribute is deleted or its value_type is updated, and
// are consumed asynchronously by the profile data migration worker.
type SchemaChangeJob struct {
	// OrgId is the organisation whose profile rows must be updated.
	OrgId string `json:"org_id"`

	// Scope is one of "identity_attributes", "traits", or "application_data".
	Scope string `json:"scope"`

	// KeyPath is the JSON path within the scope's JSONB column.
	// A single-level attribute like "traits.interests" yields ["interests"].
	// A nested attribute like "traits.address.city" yields ["address", "city"].
	KeyPath []string `json:"key_path"`

	// ChangeType is ChangeTypeDeleted or ChangeTypeTypeChanged.
	ChangeType string `json:"change_type"`

	// OldValueType and NewValueType are set only for ChangeTypeTypeChanged.
	OldValueType string `json:"old_value_type,omitempty"`
	NewValueType string `json:"new_value_type,omitempty"`

	// AppId is set only when Scope is "application_data".
	AppId string `json:"app_id,omitempty"`
}
