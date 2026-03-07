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

package service

import (
	"testing"

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

const testOrg = "test-org"

// attr is a shorthand constructor for test schema attributes.
func attr(name, valueType string) model.ProfileSchemaAttribute {
	return model.ProfileSchemaAttribute{AttributeName: name, ValueType: valueType}
}

// attrWithURI builds an attribute whose name is a claim URI (as received from IS).
func attrWithURI(uri, valueType string) model.ProfileSchemaAttribute {
	return model.ProfileSchemaAttribute{AttributeName: uri, ValueType: valueType}
}

func TestComputeIdentityAttrDiff_NoChange(t *testing.T) {
	current := []model.ProfileSchemaAttribute{
		attr("email", constants.StringDataType),
		attr("phone_number", constants.StringDataType),
	}
	incoming := []model.ProfileSchemaAttribute{
		attrWithURI("http://wso2.org/claims/email", constants.StringDataType),
		attrWithURI("http://wso2.org/claims/phone_number", constants.StringDataType),
	}

	jobs := ComputeIdentityAttrDiff(testOrg, current, incoming)
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %d: %+v", len(jobs), jobs)
	}
}

func TestComputeIdentityAttrDiff_DeletedAttribute(t *testing.T) {
	current := []model.ProfileSchemaAttribute{
		attr("email", constants.StringDataType),
		attr("phone_number", constants.StringDataType),
	}
	// phone_number is gone from the incoming set.
	incoming := []model.ProfileSchemaAttribute{
		attrWithURI("http://wso2.org/claims/email", constants.StringDataType),
	}

	jobs := ComputeIdentityAttrDiff(testOrg, current, incoming)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d: %+v", len(jobs), jobs)
	}
	j := jobs[0]
	if j.ChangeType != model.ChangeTypeDeleted {
		t.Errorf("expected ChangeTypeDeleted, got %q", j.ChangeType)
	}
	if len(j.KeyPath) != 1 || j.KeyPath[0] != "phone_number" {
		t.Errorf("unexpected key path %v", j.KeyPath)
	}
	if j.Scope != constants.IdentityAttributes {
		t.Errorf("unexpected scope %q", j.Scope)
	}
}

func TestComputeIdentityAttrDiff_TypeChanged(t *testing.T) {
	current := []model.ProfileSchemaAttribute{
		attr("age", constants.StringDataType),
	}
	incoming := []model.ProfileSchemaAttribute{
		attrWithURI("http://wso2.org/claims/age", constants.IntegerDataType),
	}

	jobs := ComputeIdentityAttrDiff(testOrg, current, incoming)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d: %+v", len(jobs), jobs)
	}
	j := jobs[0]
	if j.ChangeType != model.ChangeTypeTypeChanged {
		t.Errorf("expected ChangeTypeTypeChanged, got %q", j.ChangeType)
	}
	if j.OldValueType != constants.StringDataType {
		t.Errorf("expected old type %q, got %q", constants.StringDataType, j.OldValueType)
	}
	if j.NewValueType != constants.IntegerDataType {
		t.Errorf("expected new type %q, got %q", constants.IntegerDataType, j.NewValueType)
	}
	if len(j.KeyPath) != 1 || j.KeyPath[0] != "age" {
		t.Errorf("unexpected key path %v", j.KeyPath)
	}
}

func TestComputeIdentityAttrDiff_MultipleChanges(t *testing.T) {
	current := []model.ProfileSchemaAttribute{
		attr("email", constants.StringDataType),
		attr("age", constants.StringDataType),
		attr("phone_number", constants.StringDataType),
	}
	// email: no change; age: type changed; phone_number: deleted
	incoming := []model.ProfileSchemaAttribute{
		attrWithURI("http://wso2.org/claims/email", constants.StringDataType),
		attrWithURI("http://wso2.org/claims/age", constants.IntegerDataType),
	}

	jobs := ComputeIdentityAttrDiff(testOrg, current, incoming)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d: %+v", len(jobs), jobs)
	}

	byKey := make(map[string]model.SchemaChangeJob)
	for _, j := range jobs {
		byKey[j.KeyPath[0]] = j
	}

	if j, ok := byKey["phone_number"]; !ok || j.ChangeType != model.ChangeTypeDeleted {
		t.Errorf("expected phone_number deleted job, got %+v", byKey["phone_number"])
	}
	if j, ok := byKey["age"]; !ok || j.ChangeType != model.ChangeTypeTypeChanged {
		t.Errorf("expected age type-changed job, got %+v", byKey["age"])
	}
}

func TestComputeIdentityAttrDiff_EmptyCurrent(t *testing.T) {
	jobs := ComputeIdentityAttrDiff(testOrg, nil, []model.ProfileSchemaAttribute{
		attrWithURI("http://wso2.org/claims/email", constants.StringDataType),
	})
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs when current is empty, got %d", len(jobs))
	}
}

func TestComputeIdentityAttrDiff_MultiValuedChanged(t *testing.T) {
	current := []model.ProfileSchemaAttribute{
		{AttributeName: "interests", ValueType: constants.StringDataType, MultiValued: false},
	}
	// Same value_type but multi_valued flipped true — stored scalar is now incompatible.
	incoming := []model.ProfileSchemaAttribute{
		{AttributeName: "http://wso2.org/claims/interests", ValueType: constants.StringDataType, MultiValued: true},
	}

	jobs := ComputeIdentityAttrDiff(testOrg, current, incoming)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job for multi_valued change, got %d: %+v", len(jobs), jobs)
	}
	if jobs[0].ChangeType != model.ChangeTypeScalarToArray {
		t.Errorf("expected ChangeTypeScalarToArray, got %q", jobs[0].ChangeType)
	}
}

func TestComputeIdentityAttrDiff_MultiValuedUnchanged(t *testing.T) {
	current := []model.ProfileSchemaAttribute{
		{AttributeName: "tags", ValueType: constants.StringDataType, MultiValued: true},
	}
	incoming := []model.ProfileSchemaAttribute{
		{AttributeName: "http://wso2.org/claims/tags", ValueType: constants.StringDataType, MultiValued: true},
	}

	jobs := ComputeIdentityAttrDiff(testOrg, current, incoming)
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs when multi_valued is unchanged, got %d: %+v", len(jobs), jobs)
	}
}

func TestComputeIdentityAttrDiff_EmptyIncoming(t *testing.T) {
	current := []model.ProfileSchemaAttribute{
		attr("email", constants.StringDataType),
		attr("phone_number", constants.StringDataType),
	}
	jobs := ComputeIdentityAttrDiff(testOrg, current, nil)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 deleted jobs, got %d", len(jobs))
	}
	for _, j := range jobs {
		if j.ChangeType != model.ChangeTypeDeleted {
			t.Errorf("expected ChangeTypeDeleted for all jobs, got %q", j.ChangeType)
		}
	}
}

// --- keyPathFromAttributeName ---

func TestKeyPathFromAttributeName(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{"traits.interests", []string{"interests"}},
		{"traits.address.city", []string{"address", "city"}},
		{"identity_attributes.email", []string{"email"}},
		{"application_data.device_id", []string{"device_id"}},
		{"traits", nil},
		{"", nil},
	}

	for _, tc := range cases {
		got := keyPathFromAttributeName(tc.input)
		if len(got) != len(tc.expected) {
			t.Errorf("keyPathFromAttributeName(%q): got %v, want %v", tc.input, got, tc.expected)
			continue
		}
		for i := range got {
			if got[i] != tc.expected[i] {
				t.Errorf("keyPathFromAttributeName(%q)[%d]: got %q, want %q", tc.input, i, got[i], tc.expected[i])
			}
		}
	}
}

// --- SchemaChangeJobForDelete ---

func TestSchemaChangeJobForDelete(t *testing.T) {
	a := model.ProfileSchemaAttribute{
		AttributeName:         "traits.score",
		ValueType:             constants.IntegerDataType,
		ApplicationIdentifier: "",
	}
	job := SchemaChangeJobForDelete(testOrg, a)
	if job.ChangeType != model.ChangeTypeDeleted {
		t.Errorf("expected ChangeTypeDeleted, got %q", job.ChangeType)
	}
	if job.Scope != constants.Traits {
		t.Errorf("expected scope %q, got %q", constants.Traits, job.Scope)
	}
	if len(job.KeyPath) != 1 || job.KeyPath[0] != "score" {
		t.Errorf("unexpected KeyPath %v", job.KeyPath)
	}
}

func TestSchemaChangeJobForDelete_AppData(t *testing.T) {
	a := model.ProfileSchemaAttribute{
		AttributeName:         "application_data.device_id",
		ValueType:             constants.StringDataType,
		ApplicationIdentifier: "app-123",
	}
	job := SchemaChangeJobForDelete(testOrg, a)
	if job.Scope != constants.ApplicationData {
		t.Errorf("expected scope %q, got %q", constants.ApplicationData, job.Scope)
	}
	if job.AppId != "app-123" {
		t.Errorf("expected AppId app-123, got %q", job.AppId)
	}
}

// --- SchemaChangeJobForComplexSubAttrRemoved ---

func TestSchemaChangeJobForComplexSubAttrRemoved(t *testing.T) {
	parent := model.ProfileSchemaAttribute{
		AttributeName:         "traits.address",
		ValueType:             "complex",
		ApplicationIdentifier: "",
	}
	removedSub := model.SubAttribute{
		AttributeId:   "sub-1",
		AttributeName: "traits.address.city",
	}
	job := SchemaChangeJobForComplexSubAttrRemoved(testOrg, parent, removedSub)

	if job.ChangeType != model.ChangeTypeComplexSubAttrRemoved {
		t.Errorf("expected ChangeTypeComplexSubAttrRemoved, got %q", job.ChangeType)
	}
	if job.OrgId != testOrg {
		t.Errorf("expected org %q, got %q", testOrg, job.OrgId)
	}
	if job.Scope != "traits" {
		t.Errorf("expected scope %q, got %q", "traits", job.Scope)
	}
	if len(job.KeyPath) != 2 || job.KeyPath[0] != "address" || job.KeyPath[1] != "city" {
		t.Errorf("expected KeyPath [address city], got %v", job.KeyPath)
	}
	if job.AppId != "" {
		t.Errorf("expected empty AppId, got %q", job.AppId)
	}
}

func TestSchemaChangeJobForComplexSubAttrRemoved_AppData(t *testing.T) {
	parent := model.ProfileSchemaAttribute{
		AttributeName:         "application_data.contact",
		ApplicationIdentifier: "app-42",
	}
	removedSub := model.SubAttribute{
		AttributeId:   "sub-2",
		AttributeName: "application_data.contact.email",
	}
	job := SchemaChangeJobForComplexSubAttrRemoved(testOrg, parent, removedSub)

	if job.ChangeType != model.ChangeTypeComplexSubAttrRemoved {
		t.Errorf("expected ChangeTypeComplexSubAttrRemoved, got %q", job.ChangeType)
	}
	if job.Scope != "application_data" {
		t.Errorf("expected scope application_data, got %q", job.Scope)
	}
	if len(job.KeyPath) != 2 || job.KeyPath[0] != "contact" || job.KeyPath[1] != "email" {
		t.Errorf("expected KeyPath [contact email], got %v", job.KeyPath)
	}
	if job.AppId != "app-42" {
		t.Errorf("expected AppId app-42, got %q", job.AppId)
	}
}

// --- SchemaChangeJobForComplexSubAttrAdded ---

func TestSchemaChangeJobForComplexSubAttrAdded(t *testing.T) {
	parent := model.ProfileSchemaAttribute{
		AttributeName:         "traits.address",
		ApplicationIdentifier: "",
	}
	addedSub := model.SubAttribute{
		AttributeId:   "sub-3",
		AttributeName: "traits.address.zip",
	}
	job := SchemaChangeJobForComplexSubAttrAdded(testOrg, parent, addedSub)

	if job.ChangeType != model.ChangeTypeComplexSubAttrAdded {
		t.Errorf("expected ChangeTypeComplexSubAttrAdded, got %q", job.ChangeType)
	}
	if job.OrgId != testOrg {
		t.Errorf("expected org %q, got %q", testOrg, job.OrgId)
	}
	if job.Scope != "traits" {
		t.Errorf("expected scope traits, got %q", job.Scope)
	}
	if len(job.KeyPath) != 2 || job.KeyPath[0] != "address" || job.KeyPath[1] != "zip" {
		t.Errorf("expected KeyPath [address zip], got %v", job.KeyPath)
	}
}

func TestSchemaChangeJobForComplexSubAttrAdded_AppData(t *testing.T) {
	parent := model.ProfileSchemaAttribute{
		AttributeName:         "application_data.prefs",
		ApplicationIdentifier: "app-99",
	}
	addedSub := model.SubAttribute{
		AttributeId:   "sub-4",
		AttributeName: "application_data.prefs.theme",
	}
	job := SchemaChangeJobForComplexSubAttrAdded(testOrg, parent, addedSub)

	if job.Scope != "application_data" {
		t.Errorf("expected scope application_data, got %q", job.Scope)
	}
	if job.AppId != "app-99" {
		t.Errorf("expected AppId app-99, got %q", job.AppId)
	}
	if len(job.KeyPath) != 2 || job.KeyPath[0] != "prefs" || job.KeyPath[1] != "theme" {
		t.Errorf("expected KeyPath [prefs theme], got %v", job.KeyPath)
	}
}

// --- SchemaChangeJobForTypeChange ---

func TestSchemaChangeJobForTypeChange(t *testing.T) {
	a := model.ProfileSchemaAttribute{
		AttributeName: "traits.score",
		ValueType:     constants.StringDataType,
	}
	job := SchemaChangeJobForTypeChange(testOrg, a, constants.IntegerDataType)
	if job.ChangeType != model.ChangeTypeTypeChanged {
		t.Errorf("expected ChangeTypeTypeChanged, got %q", job.ChangeType)
	}
	if job.OldValueType != constants.StringDataType {
		t.Errorf("expected old type %q, got %q", constants.StringDataType, job.OldValueType)
	}
	if job.NewValueType != constants.IntegerDataType {
		t.Errorf("expected new type %q, got %q", constants.IntegerDataType, job.NewValueType)
	}
}
