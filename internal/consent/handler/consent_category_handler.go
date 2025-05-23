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

package handler

import (
	"encoding/json"
	consentModel "github.com/wso2/identity-customer-data-service/internal/consent/model"
	"github.com/wso2/identity-customer-data-service/internal/consent/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"net/http"
	"strings"
)

type ConsentCategoryHandler struct{}

func NewConsentCategoryHandler() *ConsentCategoryHandler {
	return &ConsentCategoryHandler{}
}

// GetAllConsentCategories handles GET /consent-categories
func (h *ConsentCategoryHandler) GetAllConsentCategories(w http.ResponseWriter, r *http.Request) {

	service := provider.NewConsentCategoryProvider().GetConsentCategoryService()
	categories, err := service.GetAllConsentCategories()
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(categories)
}

// AddConsentCategory handles POST /consent-categories
func (h *ConsentCategoryHandler) AddConsentCategory(w http.ResponseWriter, r *http.Request) {

	var category consentModel.ConsentCategory
	if err := json.NewDecoder(r.Body).Decode(&category); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	service := provider.NewConsentCategoryProvider().GetConsentCategoryService()
	if err := service.AddConsentCategory(category); err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(category)
}

// GetConsentCategory handles GET /consent-categories/{id}
func (h *ConsentCategoryHandler) GetConsentCategory(w http.ResponseWriter, r *http.Request) {

	categoryId := extractLastPathSegment(r.URL.Path)
	if categoryId == "" {
		http.Error(w, "Category ID is required", http.StatusBadRequest)
		return
	}

	service := provider.NewConsentCategoryProvider().GetConsentCategoryService()
	category, err := service.GetConsentCategory(categoryId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(category)
}

// UpdateConsentCategory handles PUT /consent-categories/{id}
func (h *ConsentCategoryHandler) UpdateConsentCategory(w http.ResponseWriter, r *http.Request) {

	categoryId := extractLastPathSegment(r.URL.Path)
	if categoryId == "" {
		http.Error(w, "Category ID is required", http.StatusBadRequest)
		return
	}

	var category consentModel.ConsentCategory
	if err := json.NewDecoder(r.Body).Decode(&category); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	service := provider.NewConsentCategoryProvider().GetConsentCategoryService()
	if err := service.UpdateConsentCategory(category); err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(category)
}

// DeleteConsentCategory handles Delete /consent-categories/{id}
func (h *ConsentCategoryHandler) DeleteConsentCategory(w http.ResponseWriter, r *http.Request) {

	categoryId := extractLastPathSegment(r.URL.Path)
	if categoryId == "" {
		http.Error(w, "Category ID is required", http.StatusBadRequest)
		return
	}

	var category consentModel.ConsentCategory
	if err := json.NewDecoder(r.Body).Decode(&category); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	service := provider.NewConsentCategoryProvider().GetConsentCategoryService()
	if err := service.UpdateConsentCategory(category); err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(category)
}

func extractLastPathSegment(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
