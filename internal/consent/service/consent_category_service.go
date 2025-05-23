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
	"github.com/google/uuid"
	model "github.com/wso2/identity-customer-data-service/internal/consent/model"
	"github.com/wso2/identity-customer-data-service/internal/consent/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"net/http"
)

// ConsentCategoryServiceInterface defines the service interface.
type ConsentCategoryServiceInterface interface {
	GetAllConsentCategories() ([]model.ConsentCategory, error)
	GetConsentCategory(id string) (*model.ConsentCategory, error)
	AddConsentCategory(category model.ConsentCategory) (*model.ConsentCategory, error)
	UpdateConsentCategory(category model.ConsentCategory) error
	DeleteConsentCategory(id string) error
}

// ConsentCategoryService is the default implementation.
type ConsentCategoryService struct{}

// GetConsentCategoryService returns a new instance.
func GetConsentCategoryService() ConsentCategoryServiceInterface {
	return &ConsentCategoryService{}
}

// GetAllConsentCategories retrieves all categories.
func (cs *ConsentCategoryService) GetAllConsentCategories() ([]model.ConsentCategory, error) {

	consentCat, err := store.GetAllConsentCategories()

	if err != nil {
		return nil, err
	}
	if len(consentCat) == 0 {
		return []model.ConsentCategory{}, nil
	}
	return consentCat, nil

}

// GetConsentCategory retrieves a category by ID.
func (cs *ConsentCategoryService) GetConsentCategory(id string) (*model.ConsentCategory, error) {

	consentCat, err := store.GetConsentCategoryByID(id)
	if err != nil {
		return nil, err
	}
	if consentCat == nil {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CONSENT_CAT_NOT_FOUND.Code,
			Message:     errors2.CONSENT_CAT_NOT_FOUND.Message,
			Description: "Consent category not found.",
		}, http.StatusNotFound)
	}
	return consentCat, nil
}

// AddConsentCategory adds a new category.
func (cs *ConsentCategoryService) AddConsentCategory(category model.ConsentCategory) (*model.ConsentCategory, error) {

	err, isValid := cs.validateConsentCat(category)

	if !isValid || err != nil {
		return nil, err
	}

	existingCat, err := store.GetConsentCategoryByName(category.CategoryName)

	if err != nil {
		return nil, err
	}

	if existingCat != nil && existingCat.CategoryName == category.CategoryName {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CONSENT_CAT_ALREADY_EXISTS.Code,
			Message:     errors2.CONSENT_CAT_ALREADY_EXISTS.Message,
			Description: "Category with the same name already exists.",
		}, http.StatusConflict)
	}

	if category.CategoryIdentifier == "" {
		category.CategoryIdentifier = uuid.New().String()
	}

	err = store.AddConsentCategory(category)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (cs *ConsentCategoryService) validateConsentCat(category model.ConsentCategory) (error, bool) {

	if category.CategoryName == "" || category.Purpose == "" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CONSENT_CAT_VALIDATION.Code,
			Message:     errors2.CONSENT_CAT_VALIDATION.Message,
			Description: "category_name, category_identifier, and purpose are required.",
		}, http.StatusBadRequest), false
	}

	// Loop through the purposes and check if they are allowed
	if len(category.Purpose) == 0 {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CONSENT_CAT_VALIDATION.Code,
			Message:     errors2.CONSENT_CAT_VALIDATION.Message,
			Description: "Purpose is required.",
		}, http.StatusBadRequest), false
	}

	if !constants.AllowedConsentPurposes[category.Purpose] {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.CONSENT_CAT_VALIDATION.Code,
			Message:     errors2.CONSENT_CAT_VALIDATION.Message,
			Description: "Invalid purpose provided. Allowed values are profiling, personalization, destination.",
		}, http.StatusBadRequest), false
	}
	return nil, true
}

// UpdateConsentCategory updates an existing category.
func (cs *ConsentCategoryService) UpdateConsentCategory(category model.ConsentCategory) error {

	if category.CategoryIdentifier == "" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "Consent category ID is required for update.",
		}, http.StatusBadRequest)
	}
	return store.UpdateConsentCategory(category)
}

// UpdateConsentCategory updates an existing category.
func (cs *ConsentCategoryService) DeleteConsentCategory(categoryId string) error {
	if categoryId == "" {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "Consent category ID is required for update.",
		}, http.StatusBadRequest)
	}
	return store.DeleteConsentCategory(categoryId)
}
