/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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

package store

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func UpsertBlockingKeys(profileID, orgHandle string, keys []model.BlockingKey) error {
	logger := log.GetLogger()
	if len(keys) == 0 {
		return nil
	}

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		logger.Error("BlockingStore: failed to get DB client", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: "Failed to connect to database for blocking key indexing.",
		}, err)
	}
	defer dbClient.Close()

	tx, err := dbClient.BeginTx()
	if err != nil {
		logger.Error("BlockingStore: failed to begin transaction", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to begin transaction for blocking key upsert: %s", profileID),
		}, err)
	}

	deleteQuery := scripts.DeleteBlockingKeysSQL[provider.NewDBProvider().GetDBType()]
	if _, err = tx.Exec(deleteQuery, profileID); err != nil {
		logger.Error("BlockingStore: failed to delete existing blocking keys", log.Error(err))
		if rbErr := tx.Rollback(); rbErr != nil {
			logger.Error("BlockingStore: failed to rollback transaction", log.Error(rbErr))
		}
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to delete existing blocking keys for profile: %s", profileID),
		}, err)
	}

	var valueClauses []string
	var args []interface{}
	argIdx := 1

	for _, key := range keys {
		valueClauses = append(valueClauses,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4))
		args = append(args, uuid.New().String(), profileID, orgHandle, key.AttributeName, key.KeyValue)
		argIdx += 5
	}

	insertQuery := fmt.Sprintf(
		scripts.IRInsertBlockingKeys[provider.NewDBProvider().GetDBType()],
		strings.Join(valueClauses, ", "),
	)

	if _, err = tx.Exec(insertQuery, args...); err != nil {
		logger.Error("BlockingStore: failed to insert blocking keys", log.Error(err))
		if rbErr := tx.Rollback(); rbErr != nil {
			logger.Error("BlockingStore: failed to rollback transaction", log.Error(rbErr))
		}
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to insert blocking keys for profile: %s", profileID),
		}, err)
	}

	if err = tx.Commit(); err != nil {
		logger.Error("BlockingStore: failed to commit transaction", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to commit blocking key upsert for profile: %s", profileID),
		}, err)
	}

	logger.Info(fmt.Sprintf("BlockingStore: indexed %d blocking keys for profile '%s'", len(keys), profileID))
	return nil
}

func DeleteBlockingKeys(profileID string) error {
	logger := log.GetLogger()

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: "Failed to connect to database for blocking key deletion.",
		}, err)
	}
	defer dbClient.Close()

	deleteQuery := scripts.DeleteBlockingKeysSQL[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(deleteQuery, profileID)
	if err != nil {
		logger.Error("BlockingStore: failed to delete blocking keys", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to delete blocking keys for profile: %s", profileID),
		}, err)
	}

	return nil
}

// DeleteBlockingKeysByAttribute removes all blocking keys for a specific attribute across all profiles in an org.
func DeleteBlockingKeysByAttribute(orgHandle, attributeName string) error {
	logger := log.GetLogger()

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: "Failed to connect to database for attribute blocking key deletion.",
		}, err)
	}
	defer dbClient.Close()

	query := scripts.DeleteBlockingKeysByAttributeSQL[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, orgHandle, attributeName)
	if err != nil {
		logger.Error(fmt.Sprintf("BlockingStore: failed to delete blocking keys for attribute '%s'", attributeName),
			log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to delete blocking keys for attribute: %s", attributeName),
		}, err)
	}

	logger.Info(fmt.Sprintf("BlockingStore: deleted blocking keys for attribute '%s' in org '%s'",
		attributeName, orgHandle))
	return nil
}

// InsertBlockingKeys inserts blocking keys for a profile without deleting existing keys.
// Uses ON CONFLICT DO NOTHING to skip duplicates.
func InsertBlockingKeys(profileID, orgHandle string, keys []model.BlockingKey) error {
	logger := log.GetLogger()
	if len(keys) == 0 {
		return nil
	}

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: "Failed to connect to database for blocking key insertion.",
		}, err)
	}
	defer dbClient.Close()

	var valueClauses []string
	var args []interface{}
	argIdx := 1

	for _, key := range keys {
		valueClauses = append(valueClauses,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4))
		args = append(args, uuid.New().String(), profileID, orgHandle, key.AttributeName, key.KeyValue)
		argIdx += 5
	}

	insertQuery := fmt.Sprintf(
		scripts.IRInsertBlockingKeys[provider.NewDBProvider().GetDBType()],
		strings.Join(valueClauses, ", "),
	)

	_, err = dbClient.ExecuteQuery(insertQuery, args...)
	if err != nil {
		logger.Error("BlockingStore: failed to insert blocking keys", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to insert blocking keys for profile: %s", profileID),
		}, err)
	}

	return nil
}

func FindCandidateIDsByKeys(
	orgHandle, attributeName string,
	keyValues []string,
	excludeProfileID string,
	maxResults int,
) ([]string, error) {
	logger := log.GetLogger()

	if len(keyValues) == 0 {
		return []string{}, nil
	}

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: "Failed to connect to database for blocking key lookup.",
		}, err)
	}
	defer dbClient.Close()

	args := []interface{}{orgHandle, attributeName}
	var inClauses []string
	argIdx := 3

	for _, kv := range keyValues {
		inClauses = append(inClauses, fmt.Sprintf("$%d", argIdx))
		args = append(args, kv)
		argIdx++
	}

	args = append(args, excludeProfileID)
	excludeArgIdx := argIdx
	argIdx++

	args = append(args, maxResults+1)
	limitArgIdx := argIdx

	query := fmt.Sprintf(
		scripts.IRFindCandidateIDsByKeys[provider.NewDBProvider().GetDBType()],
		strings.Join(inClauses, ", "),
		excludeArgIdx,
		limitArgIdx,
	)

	results, err := dbClient.ExecuteQuery(query, args...)
	if err != nil {
		logger.Error("BlockingStore: failed to query blocking keys", log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_BLOCKING_KEYS_FAILED.Code,
			Message:     errors2.IR_BLOCKING_KEYS_FAILED.Message,
			Description: fmt.Sprintf("Failed to query blocking keys for attribute: %s", attributeName),
		}, err)
	}

	if len(results) > maxResults {
		return nil, nil
	}

	profileIDs := make([]string, 0, len(results))
	for _, row := range results {
		if id, ok := row["profile_id"].(string); ok && id != "" {
			profileIDs = append(profileIDs, id)
		}
	}

	return profileIDs, nil
}

func GetProfilesByIDs(profileIDs []string) ([]model.ProfileData, error) {
	logger := log.GetLogger()

	if len(profileIDs) == 0 {
		return []model.ProfileData{}, nil
	}

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: "Failed to connect to database for profile batch lookup.",
		}, err)
	}
	defer dbClient.Close()

	var inClauses []string
	var args []interface{}
	for i, id := range profileIDs {
		inClauses = append(inClauses, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}

	query := fmt.Sprintf(
		scripts.IRGetProfilesByIDs[provider.NewDBProvider().GetDBType()],
		strings.Join(inClauses, ", "),
	)

	results, err := dbClient.ExecuteQuery(query, args...)
	if err != nil {
		logger.Error("BlockingStore: failed to batch load profiles", log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: "Failed to batch load candidate profiles.",
		}, err)
	}

	profiles := make([]model.ProfileData, 0, len(results))
	for _, row := range results {
		pd, err := scanProfileData(row)
		if err != nil {
			logger.Warn("BlockingStore: skipping profile due to scan error", log.Error(err))
			continue
		}
		profiles = append(profiles, pd)
	}

	logger.Info(fmt.Sprintf("BlockingStore: loaded %d profiles by ID", len(profiles)))
	return profiles, nil
}
