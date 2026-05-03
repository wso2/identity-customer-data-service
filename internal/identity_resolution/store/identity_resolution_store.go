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
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/database/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/database/scripts"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

func GetProfilesForOrg(orgHandle string) ([]model.ProfileData, error) {
	logger := log.GetLogger()
	logger.Info("Store: loading profiles for org", log.String("orgHandle", orgHandle))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		logger.Error("Store: failed to get DB client", log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: "Failed to connect to database for profile lookup.",
		}, err)
	}
	defer dbClient.Close()

	query := scripts.IRGetProfilesForOrg[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, orgHandle)
	if err != nil {
		logger.Error("Store: failed to query profiles", log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: fmt.Sprintf("Failed to load profiles for org: %s", orgHandle),
		}, err)
	}

	profiles := make([]model.ProfileData, 0, len(results))
	for _, row := range results {
		pd, err := scanProfileData(row)
		if err != nil {
			logger.Warn("Store: skipping profile due to scan error", log.Error(err))
			continue
		}
		profiles = append(profiles, pd)
	}

	logger.Info(fmt.Sprintf("Store: loaded %d profiles for org '%s'", len(profiles), orgHandle))
	return profiles, nil
}

func GetProfilesForOrgPaginated(orgHandle string, limit, offset int) ([]model.ProfileData, error) {
	logger := log.GetLogger()

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: "Failed to connect to database for paginated profile lookup.",
		}, err)
	}
	defer dbClient.Close()

	query := scripts.IRGetProfilesForOrgPaginated[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, orgHandle, limit, offset)
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: fmt.Sprintf("Failed to load profiles (offset=%d) for org: %s", offset, orgHandle),
		}, err)
	}

	profiles := make([]model.ProfileData, 0, len(results))
	for _, row := range results {
		pd, err := scanProfileData(row)
		if err != nil {
			logger.Warn("Store: skipping profile due to scan error", log.Error(err))
			continue
		}
		profiles = append(profiles, pd)
	}

	return profiles, nil
}

func GetProfileByID(profileID string) (*model.ProfileData, error) {
	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: "Failed to connect to database for profile lookup.",
		}, err)
	}
	defer dbClient.Close()

	query := scripts.IRGetProfileByID[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, profileID)
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: fmt.Sprintf("Failed to load profile: %s", profileID),
		}, err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	pd, err := scanProfileData(results[0])
	if err != nil {
		return nil, err
	}
	return &pd, nil
}

func scanProfileData(row map[string]interface{}) (model.ProfileData, error) {
	pd := model.ProfileData{
		Attributes: make(map[string]interface{}),
	}

	if v, ok := row["profile_id"]; ok && v != nil {
		pd.ProfileID = fmt.Sprintf("%v", v)
	}
	if v, ok := row["user_id"]; ok && v != nil {
		pd.UserID = fmt.Sprintf("%v", v)
	}
	if v, ok := row["org_handle"]; ok && v != nil {
		pd.OrgHandle = fmt.Sprintf("%v", v)
	}
	if v, ok := row["reference_profile_id"]; ok && v != nil {
		pd.ReferenceProfileID = fmt.Sprintf("%v", v)
	}

	if traitsRaw, ok := row["traits"]; ok && traitsRaw != nil {
		var traits map[string]interface{}
		if b, ok := traitsRaw.([]byte); ok {
			if err := json.Unmarshal(b, &traits); err == nil {
				model.FlattenMap("traits", traits, pd.Attributes)
			}
		}
	}

	if idAttrsRaw, ok := row["identity_attributes"]; ok && idAttrsRaw != nil {
		var idAttrs map[string]interface{}
		if b, ok := idAttrsRaw.([]byte); ok {
			if err := json.Unmarshal(b, &idAttrs); err == nil {
				model.FlattenMap("identity_attributes", idAttrs, pd.Attributes)
			}
		}
	}

	return pd, nil
}

func InsertReviewTask(task model.ReviewTask) error {
	logger := log.GetLogger()
	logger.Info("Store: inserting review task",
		log.String("incoming", task.IncomingProfileID),
		log.String("candidate", task.CandidateProfileID))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: "Failed to connect to database.",
		}, err)
	}
	defer dbClient.Close()

	// Mirror check: if the reverse pair already exists as PENDING,
	// update its score/breakdown to reflect the latest data instead of creating a duplicate.
	mirrorQuery := scripts.IRMirrorReviewTaskExists[provider.NewDBProvider().GetDBType()]
	mirrorRows, err := dbClient.ExecuteQuery(mirrorQuery,
		task.CandidateProfileID, task.IncomingProfileID, constants.ReviewStatusPending)
	if err == nil && len(mirrorRows) > 0 {
		if cnt, ok := mirrorRows[0]["count"]; ok {
			var count int
			switch c := cnt.(type) {
			case int64:
				count = int(c)
			case float64:
				count = int(c)
			}
			if count > 0 {
				// Mirror task exists. Flip it so the latest profile
				breakdownJSON, _ := json.Marshal(task.ScoreBreakdown)
				updateQuery := scripts.IRUpdateMirrorReviewTask[provider.NewDBProvider().GetDBType()]
				_, updateErr := dbClient.ExecuteQuery(updateQuery,
					task.IncomingProfileID, task.CandidateProfileID,
					task.MatchScore, string(breakdownJSON),
					task.CandidateProfileID, task.IncomingProfileID, constants.ReviewStatusPending)
				if updateErr != nil {
					logger.Warn(fmt.Sprintf("Store: failed to flip mirror task '%s' ↔ '%s'",
						task.IncomingProfileID, task.CandidateProfileID), log.Error(updateErr))
				} else {
					logger.Info(fmt.Sprintf("Store: flipped mirror task: now '%s' → '%s' (score=%.4f)",
						task.IncomingProfileID, task.CandidateProfileID, task.MatchScore))
				}
				return nil
			}
		}
	}

	breakdownJSON, _ := json.Marshal(task.ScoreBreakdown)

	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	query := scripts.IRInsertReviewTask[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query,
		task.ID, task.OrgHandle, task.IncomingProfileID, task.CandidateProfileID,
		task.MatchScore, task.Status, string(breakdownJSON))
	if err != nil {
		logger.Error("Store: failed to insert review task", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:    errors2.IR_REVIEW_TASK_FAILED.Code,
			Message: errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: fmt.Sprintf("Failed to create review task for profiles %s → %s",
				task.IncomingProfileID, task.CandidateProfileID),
		}, err)
	}

	logger.Info("Store: review task inserted successfully")
	return nil
}

// CancelRelatedReviewTasks cancels all PENDING review tasks that reference either of the given profile IDs.
// Returns the incoming profile IDs of the cancelled tasks so they can be re-evaluated.
func CancelRelatedReviewTasks(excludeTaskID, IncomingProfileID, CandidateProfileID, cancelledBy string) ([]string, error) {
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Store: cancelling related review tasks for profiles '%s' and '%s' (excluding task '%s')",
		IncomingProfileID, CandidateProfileID, excludeTaskID))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		logger.Error("Store: failed to get DB client for cascade cancel", log.Error(err))
		return nil, err
	}
	defer dbClient.Close()

	// Step 1: Find incoming profile IDs that will be affected before cancelling.
	findQuery := scripts.IRFindRelatedPendingReviewTasks[provider.NewDBProvider().GetDBType()]
	rows, err := dbClient.ExecuteQuery(findQuery,
		excludeTaskID, constants.ReviewStatusPending,
		IncomingProfileID, CandidateProfileID)
	if err != nil {
		logger.Warn("Store: failed to query related tasks for re-evaluation", log.Error(err))
		// Non-fatal for the find step — proceed with cancel anyway.
	}

	var affectedIncomingIDs []string
	for _, row := range rows {
		if v, ok := row["incoming_profile_id"]; ok && v != nil {
			id := fmt.Sprintf("%v", v)
			// Don't re-evaluate profiles that were just merged (incoming or candidate of the resolved task).
			if id != IncomingProfileID && id != CandidateProfileID {
				affectedIncomingIDs = append(affectedIncomingIDs, id)
			}
		}
	}

	// Step 2: Cancel the tasks.
	cancelQuery := scripts.IRCancelRelatedReviewTasks[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(cancelQuery,
		constants.ReviewStatusCancelled, cancelledBy,
		fmt.Sprintf("Auto-cancelled: related task %s was resolved", excludeTaskID),
		excludeTaskID, constants.ReviewStatusPending,
		IncomingProfileID, CandidateProfileID)
	if err != nil {
		logger.Error("Store: failed to cancel related review tasks", log.Error(err))
		return nil, err
	}

	logger.Info(fmt.Sprintf("Store: cancelled related tasks, %d incoming profiles eligible for re-evaluation",
		len(affectedIncomingIDs)))
	return affectedIncomingIDs, nil
}

func GetReviewTaskByID(taskID string) (*model.ReviewTask, error) {

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: "Failed to connect to database.",
		}, err)
	}
	defer dbClient.Close()

	query := scripts.IRGetReviewTaskByID[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, taskID)
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: fmt.Sprintf("Failed to load review task %s", taskID),
		}, err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	task := scanReviewTask(results[0])
	return &task, nil
}

func GetPendingReviewTasks(orgHandle string, pageSize int) ([]model.ReviewTask, int, error) {
	logger := log.GetLogger()
	logger.Info("Store: loading pending review tasks", log.String("orgHandle", orgHandle))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, 0, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: "Failed to connect to database.",
		}, err)
	}
	defer dbClient.Close()

	countQuery := scripts.IRCountPendingReviewTasks[provider.NewDBProvider().GetDBType()]
	countRows, err := dbClient.ExecuteQuery(countQuery, orgHandle, constants.ReviewStatusPending)
	if err != nil {
		return nil, 0, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: fmt.Sprintf("Failed to count review tasks for org: %s", orgHandle),
		}, err)
	}
	totalCount := 0
	if len(countRows) > 0 {
		if v, ok := countRows[0]["count"]; ok {
			switch c := v.(type) {
			case int64:
				totalCount = int(c)
			case float64:
				totalCount = int(c)
			}
		}
	}

	query := scripts.IRGetPendingReviewTasks[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, orgHandle, constants.ReviewStatusPending, pageSize)
	if err != nil {
		return nil, 0, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: fmt.Sprintf("Failed to load review tasks for org: %s", orgHandle),
		}, err)
	}

	tasks := make([]model.ReviewTask, 0, len(results))
	for _, row := range results {
		task := scanReviewTask(row)
		tasks = append(tasks, task)
	}

	logger.Info(fmt.Sprintf("Store: loaded %d pending review tasks (total %d)", len(tasks), totalCount))
	return tasks, totalCount, nil
}

func GetPendingReviewTasksByProfile(orgHandle, profileID string, pageSize int) ([]model.ReviewTask, int, error) {
	logger := log.GetLogger()
	logger.Info("Store: loading pending review tasks for profile",
		log.String("orgHandle", orgHandle), log.String("profileID", profileID))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, 0, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: "Failed to connect to database.",
		}, err)
	}
	defer dbClient.Close()

	countQuery := scripts.IRCountPendingReviewTasksByProfile[provider.NewDBProvider().GetDBType()]
	countRows, err := dbClient.ExecuteQuery(countQuery, orgHandle, constants.ReviewStatusPending, profileID)
	if err != nil {
		return nil, 0, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: fmt.Sprintf("Failed to count review tasks for profile: %s", profileID),
		}, err)
	}
	totalCount := 0
	if len(countRows) > 0 {
		if v, ok := countRows[0]["count"]; ok {
			switch c := v.(type) {
			case int64:
				totalCount = int(c)
			case float64:
				totalCount = int(c)
			}
		}
	}

	query := scripts.IRGetPendingReviewTasksByProfile[provider.NewDBProvider().GetDBType()]
	results, err := dbClient.ExecuteQuery(query, orgHandle, constants.ReviewStatusPending, profileID, pageSize)
	if err != nil {
		return nil, 0, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: fmt.Sprintf("Failed to load review tasks for profile: %s", profileID),
		}, err)
	}

	tasks := make([]model.ReviewTask, 0, len(results))
	for _, row := range results {
		task := scanReviewTask(row)
		tasks = append(tasks, task)
	}

	logger.Info(fmt.Sprintf("Store: loaded %d review tasks for profile '%s' (total %d)", len(tasks), profileID, totalCount))
	return tasks, totalCount, nil
}

func UpdateReviewTaskStatus(taskID string, status string, resolvedBy string, notes string) error {
	logger := log.GetLogger()
	logger.Info("Store: updating review task status",
		log.String("taskID", taskID), log.String("status", status))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: "Failed to connect to database.",
		}, err)
	}
	defer dbClient.Close()

	query := scripts.IRUpdateReviewTaskStatus[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, status, resolvedBy, notes, taskID)
	if err != nil {
		logger.Error("Store: failed to update review task", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_FAILED.Code,
			Message:     errors2.IR_REVIEW_TASK_FAILED.Message,
			Description: fmt.Sprintf("Failed to update review task %s to status %s", taskID, status),
		}, err)
	}

	logger.Info(fmt.Sprintf("Store: review task %s updated to %s", taskID, status))
	return nil
}

// scanReviewTask converts a DB row to a ReviewTask.
func scanReviewTask(row map[string]interface{}) model.ReviewTask {
	task := model.ReviewTask{}

	if v, ok := row["id"]; ok && v != nil {
		task.ID = fmt.Sprintf("%v", v)
	}
	if v, ok := row["org_handle"]; ok && v != nil {
		task.OrgHandle = fmt.Sprintf("%v", v)
	}
	if v, ok := row["incoming_profile_id"]; ok && v != nil {
		task.IncomingProfileID = fmt.Sprintf("%v", v)
	}
	if v, ok := row["candidate_profile_id"]; ok && v != nil {
		task.CandidateProfileID = fmt.Sprintf("%v", v)
	}
	if v, ok := row["match_score"]; ok && v != nil {
		switch f := v.(type) {
		case float64:
			task.MatchScore = f
		case []byte:
			if parsed, err := strconv.ParseFloat(string(f), 64); err == nil {
				task.MatchScore = parsed
			}
		case string:
			if parsed, err := strconv.ParseFloat(f, 64); err == nil {
				task.MatchScore = parsed
			}
		}
	}
	if v, ok := row["status"]; ok && v != nil {
		task.Status = fmt.Sprintf("%v", v)
	}
	if v, ok := row["score_breakdown"]; ok && v != nil {
		if b, ok := v.([]byte); ok {
			var breakdown map[string]float64
			if err := json.Unmarshal(b, &breakdown); err == nil {
				task.ScoreBreakdown = breakdown
			}
		}
	}
	if v, ok := row["created_at"]; ok && v != nil {
		if t, ok := v.(time.Time); ok {
			task.CreatedAt = t.UTC().Format(time.RFC3339)
		} else {
			task.CreatedAt = fmt.Sprintf("%v", v)
		}
	}
	if v, ok := row["resolved_at"]; ok && v != nil {
		if t, ok := v.(time.Time); ok {
			task.ResolvedAt = t.UTC().Format(time.RFC3339)
		} else {
			task.ResolvedAt = fmt.Sprintf("%v", v)
		}
	}
	if v, ok := row["resolved_by"]; ok && v != nil {
		task.ResolvedBy = fmt.Sprintf("%v", v)
	}
	if v, ok := row["resolution_notes"]; ok && v != nil {
		task.Notes = fmt.Sprintf("%v", v)
	}

	return task
}

func InsertMergeAuditLog(entry model.MergeAuditEntry) error {
	logger := log.GetLogger()
	logger.Info("Store: inserting merge audit log",
		log.String("primary", entry.PrimaryProfileID),
		log.String("secondary", entry.SecondaryProfileID),
		log.String("mergeType", entry.MergeType))

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		logger.Error("Store: failed to get DB client for audit log", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_AUDIT_LOG.Code,
			Message:     errors2.IR_AUDIT_LOG.Message,
			Description: "Failed to connect to database for audit log.",
		}, err)
	}
	defer dbClient.Close()

	auditID := uuid.New().String()

	query := scripts.IRInsertMergeAuditLog[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query,
		auditID, entry.OrgHandle, entry.PrimaryProfileID, entry.SecondaryProfileID,
		entry.MergeType, entry.MatchScore, entry.MergedBy)
	if err != nil {
		logger.Error("Store: failed to insert merge audit log", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_AUDIT_LOG.Code,
			Message:     errors2.IR_AUDIT_LOG.Message,
			Description: fmt.Sprintf("Failed to insert merge audit log for %s → %s", entry.PrimaryProfileID, entry.SecondaryProfileID),
		}, err)
	}

	logger.Info("Store: merge audit log inserted successfully")
	return nil
}

// InsertRejectionPair stores a rejection pair in canonical order.
func InsertRejectionPair(orgHandle, profileA, profileB, rejectedBy string) error {
	logger := log.GetLogger()

	// Canonical ordering: smaller ID is profile_id_1.
	id1, id2 := profileA, profileB
	if id1 > id2 {
		id1, id2 = id2, id1
	}

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return err
	}
	defer dbClient.Close()

	rejectionID := uuid.New().String()

	query := scripts.IRInsertRejectionPair[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, rejectionID, orgHandle, id1, id2, rejectedBy)
	if err != nil {
		logger.Error("Store: failed to insert rejection pair", log.Error(err))
		return err
	}

	return nil
}

// DeleteRejectionPairsForProfile removes all rejection pairs involving the given profile so that
// a re-evaluation triggered by a profile update can match previously rejected candidates.
func DeleteRejectionPairsForProfile(orgHandle, profileID string) error {
	logger := log.GetLogger()

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return err
	}
	defer dbClient.Close()

	query := scripts.IRDeleteRejectionPairsForProfile[provider.NewDBProvider().GetDBType()]
	_, err = dbClient.ExecuteQuery(query, orgHandle, profileID)
	if err != nil {
		logger.Warn(fmt.Sprintf("Store: failed to delete rejection pairs for profile '%s'", profileID), log.Error(err))
		return err
	}

	return nil
}

// GetRejectedProfileIDs returns the set of profile IDs that have been rejected against the given profileID.
func GetRejectedProfileIDs(orgHandle, profileID string) (map[string]bool, error) {
	logger := log.GetLogger()

	dbClient, err := provider.NewDBProvider().GetDBClient()
	if err != nil {
		return nil, err
	}
	defer dbClient.Close()

	query := scripts.IRGetRejectedProfileIDs[provider.NewDBProvider().GetDBType()]
	rows, err := dbClient.ExecuteQuery(query, orgHandle, profileID)
	if err != nil {
		logger.Warn("Store: failed to query rejection pairs", log.Error(err))
		return nil, err
	}

	rejected := make(map[string]bool)
	for _, row := range rows {
		p1 := fmt.Sprintf("%v", row["profile_id_1"])
		p2 := fmt.Sprintf("%v", row["profile_id_2"])
		if p1 == profileID {
			rejected[p2] = true
		} else {
			rejected[p1] = true
		}
	}

	if len(rejected) > 0 {
		logger.Info(fmt.Sprintf("Store: found %d rejection pairs for profile '%s'", len(rejected), profileID))
	}
	return rejected, nil
}
