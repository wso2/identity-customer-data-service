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
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// claimKeyFromURI extracts the leaf segment from a SCIM claim URI, e.g.
// "http://wso2.org/claims/emailaddress" → "emailaddress".
// This mirrors the logic in the schema store's extractClaimKeyFromURI.
func claimKeyFromURI(uri string) string {
	parts := strings.Split(strings.TrimRight(uri, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// keyPathFromAttributeName derives the JSON key path used to address a value
// inside a scope's JSONB column from the schema attribute name.
//
// Examples:
//
//	"traits.interests"      → ["interests"]
//	"traits.address.city"   → ["address", "city"]
//	"identity_attributes.email" → ["email"]
func keyPathFromAttributeName(attributeName string) []string {
	parts := strings.SplitN(attributeName, ".", 2)
	if len(parts) < 2 || parts[1] == "" {
		return nil
	}
	return strings.Split(parts[1], ".")
}

// ComputeIdentityAttrDiff compares the identity attributes currently stored in
// the DB against the incoming set fetched from the identity server and returns
// a SchemaChangeJob for each attribute that was deleted or had its value_type
// changed.
//
// current  – attributes already persisted; their AttributeName is the leaf key
//
//	(as stored by UpsertIdentityAttributes after extractClaimKeyFromURI).
//
// incoming – attributes received from the identity server; their AttributeName
//
//	is a full claim URI that must be normalised before comparison.
func ComputeIdentityAttrDiff(orgId string, current, incoming []model.ProfileSchemaAttribute) []model.SchemaChangeJob {
	// Build a map of current attributes keyed by their leaf name.
	currentMap := make(map[string]model.ProfileSchemaAttribute, len(current))
	for _, a := range current {
		currentMap[a.AttributeName] = a
	}

	// Build a map of incoming attributes keyed by their normalised leaf name.
	incomingMap := make(map[string]model.ProfileSchemaAttribute, len(incoming))
	for _, a := range incoming {
		key := claimKeyFromURI(a.AttributeName)
		if key != "" {
			incomingMap[key] = a
		}
	}

	var jobs []model.SchemaChangeJob
	for name, cur := range currentMap {
		inc, exists := incomingMap[name]
		if !exists {
			// Attribute removed from the identity server schema.
			jobs = append(jobs, model.SchemaChangeJob{
				OrgId:      orgId,
				Scope:      constants.IdentityAttributes,
				KeyPath:    []string{name},
				ChangeType: model.ChangeTypeDeleted,
			})
		} else if cur.ValueType != inc.ValueType {
			// Scalar type changed — stored values are incompatible; nullify.
			jobs = append(jobs, model.SchemaChangeJob{
				OrgId:        orgId,
				Scope:        constants.IdentityAttributes,
				KeyPath:      []string{name},
				ChangeType:   model.ChangeTypeTypeChanged,
				OldValueType: cur.ValueType,
				NewValueType: inc.ValueType,
			})
		} else if !cur.MultiValued && inc.MultiValued {
			// false → true: wrap each stored scalar in a single-element array.
			jobs = append(jobs, model.SchemaChangeJob{
				OrgId:      orgId,
				Scope:      constants.IdentityAttributes,
				KeyPath:    []string{name},
				ChangeType: model.ChangeTypeScalarToArray,
			})
		} else if cur.MultiValued && !inc.MultiValued {
			// true → false: keep the first element, discard the rest.
			jobs = append(jobs, model.SchemaChangeJob{
				OrgId:      orgId,
				Scope:      constants.IdentityAttributes,
				KeyPath:    []string{name},
				ChangeType: model.ChangeTypeArrayToScalar,
			})
		}
	}
	return jobs
}

// SchemaChangeJobForDelete builds the SchemaChangeJob to enqueue when a
// custom schema attribute (traits or application_data) is deleted via the API.
func SchemaChangeJobForDelete(orgId string, attr model.ProfileSchemaAttribute) model.SchemaChangeJob {
	scope := scopeOf(attr.AttributeName)
	return model.SchemaChangeJob{
		OrgId:      orgId,
		Scope:      scope,
		KeyPath:    keyPathFromAttributeName(attr.AttributeName),
		ChangeType: model.ChangeTypeDeleted,
		AppId:      attr.ApplicationIdentifier,
	}
}

// SchemaChangeJobForTypeChange builds the SchemaChangeJob to enqueue when a
// custom schema attribute (traits or application_data) has its value_type
// changed via the API.
func SchemaChangeJobForTypeChange(orgId string, attr model.ProfileSchemaAttribute, newValueType string) model.SchemaChangeJob {
	scope := scopeOf(attr.AttributeName)
	return model.SchemaChangeJob{
		OrgId:        orgId,
		Scope:        scope,
		KeyPath:      keyPathFromAttributeName(attr.AttributeName),
		ChangeType:   model.ChangeTypeTypeChanged,
		OldValueType: attr.ValueType,
		NewValueType: newValueType,
		AppId:        attr.ApplicationIdentifier,
	}
}

// SchemaChangeJobForScalarToArray builds the job to enqueue when an attribute's
// multi_valued changes from false to true.  The stored scalar is wrapped in a
// single-element array so no data is lost.
func SchemaChangeJobForScalarToArray(orgId string, attr model.ProfileSchemaAttribute) model.SchemaChangeJob {
	return model.SchemaChangeJob{
		OrgId:      orgId,
		Scope:      scopeOf(attr.AttributeName),
		KeyPath:    keyPathFromAttributeName(attr.AttributeName),
		ChangeType: model.ChangeTypeScalarToArray,
		AppId:      attr.ApplicationIdentifier,
	}
}

// SchemaChangeJobForArrayToScalar builds the job to enqueue when an attribute's
// multi_valued changes from true to false.  The first element of the stored
// array is kept; remaining elements are discarded.
func SchemaChangeJobForArrayToScalar(orgId string, attr model.ProfileSchemaAttribute) model.SchemaChangeJob {
	return model.SchemaChangeJob{
		OrgId:      orgId,
		Scope:      scopeOf(attr.AttributeName),
		KeyPath:    keyPathFromAttributeName(attr.AttributeName),
		ChangeType: model.ChangeTypeArrayToScalar,
		AppId:      attr.ApplicationIdentifier,
	}
}

// SchemaChangeJobForComplexSubAttrRemoved builds the job to enqueue when a
// sub-attribute is removed from a complex schema attribute while the parent
// remains complex.  KeyPath will be [parentKey, subKey], e.g. ["address", "city"].
func SchemaChangeJobForComplexSubAttrRemoved(orgId string, parentAttr model.ProfileSchemaAttribute, removedSubAttr model.SubAttribute) model.SchemaChangeJob {
	return model.SchemaChangeJob{
		OrgId:      orgId,
		Scope:      scopeOf(parentAttr.AttributeName),
		KeyPath:    keyPathFromAttributeName(removedSubAttr.AttributeName),
		ChangeType: model.ChangeTypeComplexSubAttrRemoved,
		AppId:      parentAttr.ApplicationIdentifier,
	}
}

// SchemaChangeJobForComplexSubAttrAdded builds the job to enqueue when a
// sub-attribute is added to a complex schema attribute.  Any orphaned flat
// key written by a prior removal job will be merged back into the nested
// object.  KeyPath will be [parentKey, subKey], e.g. ["address", "city"].
func SchemaChangeJobForComplexSubAttrAdded(orgId string, parentAttr model.ProfileSchemaAttribute, addedSubAttr model.SubAttribute) model.SchemaChangeJob {
	return model.SchemaChangeJob{
		OrgId:      orgId,
		Scope:      scopeOf(parentAttr.AttributeName),
		KeyPath:    keyPathFromAttributeName(addedSubAttr.AttributeName),
		ChangeType: model.ChangeTypeComplexSubAttrAdded,
		AppId:      parentAttr.ApplicationIdentifier,
	}
}

// scopeOf returns the scope segment (the first dot-separated part) of an
// attribute name, e.g. "traits.interests" → "traits".
func scopeOf(attributeName string) string {
	if i := strings.Index(attributeName, "."); i >= 0 {
		return attributeName[:i]
	}
	return attributeName
}
