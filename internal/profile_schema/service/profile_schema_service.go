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

package service

import (
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	psstr "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
)

type ProfileSchemaServiceInterface interface {
	AddProfileSchemaAttribute(rule model.ProfileSchemaAttribute) error
	GetProfileSchemaAttribute(orgId, attributeId string) (model.ProfileSchemaAttribute, error)
	PatchProfileSchemaAttribute(orgId, attributeId string, updates map[string]interface{}) error
	DeleteProfileSchemaAttribute(orgId, attributeId string) error
	GetProfileSchema(orgId string) ([]*model.ProfileSchemaAttribute, error)
	DeleteProfileSchema(orgId string) error
}

// ProfileSchemaService is the default implementation of the ProfileSchemaServiceInterface.
type ProfileSchemaService struct{}

// GetProfileSchemaService creates a new instance of UnificationRuleService.
func GetProfileSchemaService() ProfileSchemaServiceInterface {

	return &ProfileSchemaService{}
}

func (s *ProfileSchemaService) AddProfileSchemaAttribute(attr model.ProfileSchemaAttribute) error {

	// need to validate the path of the attribute and ensure its valid (identity_attribute.xyz.abc)
	// validate the attribute type
	return psstr.AddProfileSchemaAttribute(attr)
}

func (s *ProfileSchemaService) GetProfileSchemaAttribute(orgId, attributeId string) (model.ProfileSchemaAttribute, error) {
	return psstr.GetProfileSchemaAttribute(orgId, attributeId)
}

func (s *ProfileSchemaService) PatchProfileSchemaAttribute(orgId, attributeId string, updates map[string]interface{}) error {
	// need to validate the path of the attribute and ensure its valid (identity_attribute.xyz.abc)
	// validate the attribute type
	return psstr.PatchProfileSchemaAttribute(orgId, attributeId, updates)
}

func (s *ProfileSchemaService) DeleteProfileSchemaAttribute(orgId, attributeId string) error {
	return psstr.DeleteProfileSchemaAttribute(orgId, attributeId)
}

func (s *ProfileSchemaService) GetProfileSchema(orgId string) ([]*model.ProfileSchemaAttribute, error) {
	return psstr.GetProfileSchema(orgId)
}

func (s *ProfileSchemaService) DeleteProfileSchema(orgId string) error {
	return psstr.DeleteProfileSchema(orgId)
}
