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

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

var deleteBlockingKeysSQL = map[string]string{
	"postgres": `DELETE FROM blocking_keys WHERE profile_id = $1`,
}

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

	deleteQuery := deleteBlockingKeysSQL[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(deleteQuery, profileID)
	if err != nil {
		logger.Error("BlockingStore: failed to delete existing blocking keys", log.Error(err))
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
			fmt.Sprintf("($%d, $%d, $%d, $%d)", argIdx, argIdx+1, argIdx+2, argIdx+3))
		args = append(args, profileID, orgHandle, key.AttributeName, key.KeyValue)
		argIdx += 4
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO blocking_keys (profile_id, org_handle, attribute_name, key_value) VALUES %s ON CONFLICT DO NOTHING",
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

	deleteQuery := deleteBlockingKeysSQL[provider.NewDBProvider().GetDBType()]
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
		"SELECT DISTINCT profile_id FROM blocking_keys WHERE org_handle = $1 AND attribute_name = $2 AND key_value IN (%s) AND profile_id != $%d LIMIT $%d",
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
		`SELECT p.profile_id, p.user_id, p.org_handle, p.traits, p.identity_attributes,
		        pr.reference_profile_id
		 FROM profiles p
		 LEFT JOIN profile_reference pr ON p.profile_id = pr.profile_id
		 WHERE p.profile_id IN (%s) AND p.delete_profile = FALSE`,
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

func SampleAttributeValues(orgHandle, propertyName string, limit int) ([]string, error) {
	logger := log.GetLogger()

	parts := strings.SplitN(propertyName, ".", 2)
	if len(parts) < 2 {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SAMPLE_VALUES.Code,
			Message:     errors2.IR_SAMPLE_VALUES.Message,
			Description: fmt.Sprintf("Invalid property name '%s' (expected prefix.key)", propertyName),
		}, fmt.Errorf("invalid property name: %s", propertyName))
	}

	column := parts[0]
	jsonPath := parts[1]

	switch column {
	case "traits", "identity_attributes":

	default:
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SAMPLE_VALUES.Code,
			Message:     errors2.IR_SAMPLE_VALUES.Message,
			Description: fmt.Sprintf("Unsupported column '%s' for attribute sampling", column),
		}, fmt.Errorf("unsupported column: %s", column))
	}

	segments := strings.Split(jsonPath, ".")
	quotedSegments := make([]string, len(segments))
	for i, seg := range segments {
		quotedSegments[i] = fmt.Sprintf("'%s'", seg)
	}

	query := fmt.Sprintf(
		`SELECT DISTINCT jsonb_extract_path_text(%s, %s) AS val
		 FROM profiles
		 WHERE org_handle = $1
		   AND delete_profile = FALSE
		   AND jsonb_extract_path_text(%s, %s) IS NOT NULL
		   AND jsonb_extract_path_text(%s, %s) != ''
		 LIMIT $2`,
		column, strings.Join(quotedSegments, ", "),
		column, strings.Join(quotedSegments, ", "),
		column, strings.Join(quotedSegments, ", "),
	)

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SAMPLE_VALUES.Code,
			Message:     errors2.IR_SAMPLE_VALUES.Message,
			Description: "Failed to connect to database for attribute sampling.",
		}, err)
	}
	defer dbClient.Close()

	results, err := dbClient.ExecuteQuery(query, orgHandle, limit)
	if err != nil {
		logger.Warn(fmt.Sprintf("SampleAttributeValues: query failed for '%s'", propertyName), log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SAMPLE_VALUES.Code,
			Message:     errors2.IR_SAMPLE_VALUES.Message,
			Description: fmt.Sprintf("Failed to sample attribute values for '%s'", propertyName),
		}, err)
	}

	values := make([]string, 0, len(results))
	for _, row := range results {
		if v, ok := row["val"]; ok && v != nil {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				values = append(values, s)
			}
		}
	}

	logger.Debug(fmt.Sprintf("SampleAttributeValues: sampled %d values for '%s' (org=%s)",
		len(values), propertyName, orgHandle))
	return values, nil
}
