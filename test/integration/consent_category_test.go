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

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	model "github.com/wso2/identity-customer-data-service/internal/consent/model"
	"github.com/wso2/identity-customer-data-service/internal/consent/service"
)

func Test_Consent(t *testing.T) {
	svc := service.GetConsentCategoryService()

	category := model.ConsentCategory{
		CategoryName:       "Test Category",
		Purpose:            "profiling",
		CategoryIdentifier: "test-cat-001",
		Destinations:       []string{"app1", "app2"},
	}

	t.Run("Add_consent_category", func(t *testing.T) {
		created, err := svc.AddConsentCategory(category)
		assert.NoError(t, err, "Failed to add consent category")
		assert.Equal(t, category.CategoryName, created.CategoryName)
	})

	t.Run("Get_all_consent_categories", func(t *testing.T) {
		cats, err := svc.GetAllConsentCategories()
		assert.NoError(t, err)
		assert.NotEmpty(t, cats, "Expected at least one consent category")
	})

	t.Run("Get_single_category", func(t *testing.T) {
		fetched, err := svc.GetConsentCategory(category.CategoryIdentifier)
		assert.NoError(t, err)
		assert.Equal(t, category.CategoryName, fetched.CategoryName)
	})

	t.Run("Update_consent_category", func(t *testing.T) {
		category.CategoryName = "Updated Test Category"
		err := svc.UpdateConsentCategory(category)
		assert.NoError(t, err, "Failed to update consent category")

		updated, err := svc.GetConsentCategory(category.CategoryIdentifier)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Test Category", updated.CategoryName)
	})

	t.Run("Delete_consent_category", func(t *testing.T) {
		err := svc.DeleteConsentCategory(category.CategoryIdentifier)
		assert.NoError(t, err, "Failed to delete consent category")

		deleted, err := svc.GetConsentCategory(category.CategoryIdentifier)
		assert.Nil(t, deleted, "Expected category to be nil after deletion")
	})
}
