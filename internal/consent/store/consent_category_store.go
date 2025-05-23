/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package store

import (
	"fmt"
	"github.com/lib/pq"
	model "github.com/wso2/identity-customer-data-service/internal/consent/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"strings"
)

// AddConsentCategory inserts a new consent category into the database.
func AddConsentCategory(category model.ConsentCategory) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for inserting consent category: %s", category.CategoryIdentifier)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_CONSENT_CATEGORY.Code,
			Message:     errors2.ADD_CONSENT_CATEGORY.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()
	query := `INSERT INTO consent_categories (category_name, category_identifier, org_id, purpose, destinations)
				VALUES ($1, $2, $3, $4, $5)`
	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for inserting consent category: %s", category.CategoryIdentifier)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_CONSENT_CATEGORY.Code,
			Message:     errors2.ADD_CONSENT_CATEGORY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}
	_, err = tx.Exec(query, category.CategoryName, category.CategoryIdentifier, category.OrgId, category.Purpose, pq.Array(category.Destinations))
	if err != nil {
		errRollback := tx.Rollback()
		if errRollback != nil {
			errorMsg := fmt.Sprintf("Failed to rollback inserting consent category: %s", category.CategoryIdentifier)
			logger.Debug(errorMsg, log.Error(errRollback))
			return errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.ADD_CONSENT_CATEGORY.Code,
				Message:     errors2.ADD_CONSENT_CATEGORY.Message,
				Description: errorMsg,
			}, errRollback)
		}
		errorMsg := fmt.Sprintf("Failed to execute query for inserting consent category: %s", category.CategoryIdentifier)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.ADD_CONSENT_CATEGORY.Code,
			Message:     errors2.ADD_CONSENT_CATEGORY.Message,
			Description: errorMsg,
		}, err)
	}
	logger.Info(fmt.Sprintf("Successfully inserted consent category: %s", category.CategoryIdentifier))
	return tx.Commit()
}

// GetAllConsentCategories retrieves all consent categories from the database.
func GetAllConsentCategories() ([]model.ConsentCategory, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to get db client for fetching consent categories."
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_CONSENT_CATEGORIES.Code,
			Message:     errors2.FETCH_CONSENT_CATEGORIES.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	query := `SELECT category_name, category_identifier, org_id, purpose, destinations FROM consent_categories`
	results, err := dbClient.ExecuteQuery(query)
	if err != nil {
		errorMsg := "Failed to execute query for fetching consent categories."
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_CONSENT_CATEGORIES.Code,
			Message:     errors2.FETCH_CONSENT_CATEGORIES.Message,
			Description: "Failed to fetch consent categories.",
		}, err)
	}

	categories := make([]model.ConsentCategory, 0, len(results))
	for _, row := range results {
		categories = append(categories, model.ConsentCategory{
			CategoryName:       row["category_name"].(string),
			CategoryIdentifier: row["category_identifier"].(string),
			OrgId:              row["org_id"].(string),
			Purpose:            row["purpose"].(string),
			Destinations:       parseStringArray(row["destinations"]),
		})
	}
	if len(categories) == 0 {
		logger.Debug("No consent categories found")
		return nil, nil
	}
	logger.Info(fmt.Sprintf("Successfully fetched %d consent categories", len(categories)))
	return categories, nil
}

// GetConsentCategoryByID retrieves a consent category by its ID.
func GetConsentCategoryByID(id string) (*model.ConsentCategory, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for fetching consent category: %s", id)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_CONSENT_CATEGORIES.Code,
			Message:     errors2.FETCH_CONSENT_CATEGORIES.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	query := `SELECT category_name, category_identifier, org_id, purpose, destinations FROM consent_categories WHERE category_identifier = $1`
	results, err := dbClient.ExecuteQuery(query, id)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to execute query for fetching consent category: %s", id)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_CONSENT_CATEGORIES.Code,
			Message:     errors2.FETCH_CONSENT_CATEGORIES.Message,
			Description: errorMsg,
		}, err)
	}

	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("Consent category not found for id: %s", id))
		return nil, nil
	}
	row := results[0]
	category := model.ConsentCategory{
		CategoryName:       row["category_name"].(string),
		CategoryIdentifier: row["category_identifier"].(string),
		OrgId:              row["org_id"].(string),
		Purpose:            row["purpose"].(string),
		Destinations:       parseStringArray(row["destinations"]),
	}
	return &category, nil
}

// GetConsentCategoryByName retrieves a consent category by its ID.
func GetConsentCategoryByName(name string) (*model.ConsentCategory, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for fetching consent category: %s", name)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_CONSENT_CATEGORIES.Code,
			Message:     errors2.FETCH_CONSENT_CATEGORIES.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()

	query := `SELECT category_name, category_identifier, org_id, purpose, destinations FROM consent_categories WHERE category_name = $1`
	results, err := dbClient.ExecuteQuery(query, name)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to execute query for fetching consent category: %s", name)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.FETCH_CONSENT_CATEGORIES.Code,
			Message:     errors2.FETCH_CONSENT_CATEGORIES.Message,
			Description: errorMsg,
		}, err)
	}

	if len(results) == 0 {
		logger.Debug(fmt.Sprintf("Consent category not found for name: %s", name))
		return nil, nil
	}
	row := results[0]
	category := model.ConsentCategory{
		CategoryName:       row["category_name"].(string),
		CategoryIdentifier: row["category_identifier"].(string),
		OrgId:              row["org_id"].(string),
		Purpose:            row["purpose"].(string),
		Destinations:       parseStringArray(row["destinations"]),
	}
	return &category, nil
}

// UpdateConsentCategory updates an existing consent category in the database.
func UpdateConsentCategory(category model.ConsentCategory) error {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get db client for updating consent category: %s", category.CategoryIdentifier)
		logger.Debug(errorMsg, log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_CONSENT_CATEGORY.Code,
			Message:     errors2.UPDATE_CONSENT_CATEGORY.Message,
			Description: errorMsg,
		}, err)
	}
	defer dbClient.Close()
	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for updating consent category: %s",
			category.CategoryIdentifier)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_CONSENT_CATEGORY.Code,
			Message:     errors2.UPDATE_CONSENT_CATEGORY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	query := `UPDATE consent_categories SET category_name=$1, purpose=$2, destinations=$3 WHERE category_identifier=$4`
	_, err = tx.Exec(query, category.CategoryName, category.Purpose, pq.Array(category.Destinations), category.CategoryIdentifier)
	if err != nil {
		logger.Debug("Failed to update consent category", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_CONSENT_CATEGORY.Code,
			Message:     errors2.UPDATE_CONSENT_CATEGORY.Message,
			Description: "Failed to update consent category.",
		}, err)
	}
	return tx.Commit()
}

func DeleteConsentCategory(categoryId string) error {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	logger := log.GetLogger()
	if err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_CONSENT_CATEGORY.Code,
			Message:     errors2.UPDATE_CONSENT_CATEGORY.Message,
			Description: "Database connection failed.",
		}, err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to begin transaction for deleting consent category: %s", categoryId)
		logger.Debug(errorMsg, log.Error(err))
		serverError := errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_CONSENT_CATEGORY.Code,
			Message:     errors2.UPDATE_CONSENT_CATEGORY.Message,
			Description: errorMsg,
		}, err)
		return serverError
	}

	query := `DELETE FROM consent_categories WHERE category_identifier=$1`
	_, err = tx.Exec(query, categoryId)
	if err != nil {
		logger.Debug("Failed to update consent category", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.UPDATE_CONSENT_CATEGORY.Code,
			Message:     errors2.UPDATE_CONSENT_CATEGORY.Message,
			Description: "Failed to update consent category.",
		}, err)
	}
	return tx.Commit()
}

func parseStringArray(raw interface{}) []string {
	if raw == nil {
		return nil
	}

	var rawStr string
	switch v := raw.(type) {
	case []byte:
		rawStr = string(v)
	case string:
		rawStr = v
	default:
		return nil
	}

	rawStr = strings.Trim(rawStr, "{}")
	if rawStr == "" {
		return nil
	}

	items := strings.Split(rawStr, ",")
	var result []string
	for _, item := range items {
		// Trim spaces and surrounding double quotes
		clean := strings.TrimSpace(item)
		clean = strings.Trim(clean, `"`)
		result = append(result, clean)
	}

	return result
}
