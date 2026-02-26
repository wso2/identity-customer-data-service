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

package service

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	irStore "github.com/wso2/identity-customer-data-service/internal/identity_resolution/store"
	irWorker "github.com/wso2/identity-customer-data-service/internal/identity_resolution/worker"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	schemaStore "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/pagination"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	urStore "github.com/wso2/identity-customer-data-service/internal/unification_rules/store"
)

type IdentityResolutionServiceInterface interface {
	Search(orgHandle string, request *model.SearchRequest) (*model.SearchResponse, error)

	MergeProfiles(orgHandle string, request *model.MergeRequest) (*model.MergeResponse, error)

	GetPendingReviewTasks(orgHandle string, pageSize int) (*model.ReviewTaskListResponse, error)

	GetPendingReviewTasksByProfile(orgHandle string, profileID string, pageSize int) (*model.ReviewTaskListResponse, error)

	ResolveReviewTask(orgHandle string, taskID string, approved bool, resolvedBy string, notes string) error
}

type IdentityResolutionService struct{}

func GetIdentityResolutionService() IdentityResolutionServiceInterface {
	return &IdentityResolutionService{}
}

func (s *IdentityResolutionService) Search(orgHandle string, request *model.SearchRequest) (*model.SearchResponse, error) {
	logger := log.GetLogger()
	startTime := time.Now()

	logger.Info("Service: starting identity search",
		log.String("orgHandle", orgHandle),
		log.Int("maxResults", request.GetMaxResults()))

	rules, err := loadEnrichedRules(orgHandle)
	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		logger.Warn("Service: no active unification rules found",
			log.String("orgHandle", orgHandle))
		return &model.SearchResponse{
			Matches:         []model.MatchResult{},
			TotalCandidates: 0,
			ProcessingTime:  time.Since(startTime).Milliseconds(),
		}, nil
	}

	logger.Info(fmt.Sprintf("Service: loaded %d enriched rules", len(rules)))

	flatAttrs := request.FlatAttributes()

	blockingKeys := engine.GenerateProfileBlockingKeys(rules, flatAttrs)

	// Use empty string as pendingID since profile is not cached yet.
	candidateIDs := engine.FindCandidatesByIndex(blockingKeys, orgHandle, "", irStore.FindCandidateIDsByKeys)

	if len(candidateIDs) == 0 {
		logger.Info("Service: no candidates found after blocking")
		return &model.SearchResponse{
			Matches:         []model.MatchResult{},
			TotalCandidates: 0,
			ProcessingTime:  time.Since(startTime).Milliseconds(),
		}, nil
	}

	candidateProfiles, err := irStore.GetProfilesByIDs(candidateIDs)
	if err != nil {
		return nil, err
	}

	profileMap := make(map[string]*model.ProfileData, len(candidateProfiles))
	for i := range candidateProfiles {
		profileMap[candidateProfiles[i].ProfileID] = &candidateProfiles[i]
	}

	// Resolve child candidates to their master profiles so search results
	// reference the surviving (master) profile, not a merged child.
	{
		resolvedIDs := make([]string, 0, len(candidateIDs))
		seen := make(map[string]bool)
		for _, cid := range candidateIDs {
			candidate, exists := profileMap[cid]
			if !exists {
				continue
			}
			resolvedID := cid
			if candidate.IsChild() {
				masterID := candidate.ReferenceProfileID
				resolvedID = masterID
				if _, ok := profileMap[masterID]; !ok {
					masterProfiles, loadErr := irStore.GetProfilesByIDs([]string{masterID})
					if loadErr != nil || len(masterProfiles) == 0 {
						logger.Warn(fmt.Sprintf("Service: could not load master '%s' for child '%s', skipping",
							masterID, cid))
						continue
					}
					profileMap[masterID] = &masterProfiles[0]
				}
				logger.Info(fmt.Sprintf("Service: resolved child candidate '%s' → master '%s'", cid, masterID))
			}
			if !seen[resolvedID] {
				seen[resolvedID] = true
				resolvedIDs = append(resolvedIDs, resolvedID)
			}
		}
		candidateIDs = resolvedIDs
	}

	inputProfileType := model.DetermineProfileType(flatAttrs)
	thresholds := model.LoadThresholds(orgHandle)

	var matches []model.MatchResult
	for _, candidateID := range candidateIDs {
		candidate, exists := profileMap[candidateID]
		if !exists {
			continue
		}

		candidateType := candidate.GetProfileType()
		mode := model.DetermineMode(inputProfileType, candidateType, thresholds.SmartResolutionEnabled)

		finalScore, breakdown := engine.ScoreCandidate(flatAttrs, candidate, rules, mode, thresholds.AutoMerge)

		matches = append(matches, model.MatchResult{
			CandidateID:    candidateID,
			UserID:         candidate.UserID,
			FinalScore:     finalScore,
			ScoreBreakdown: breakdown,
			Attributes:     candidate.Attributes,
		})
	}

	threshold := request.GetThreshold(thresholds)
	var filteredMatches []model.MatchResult
	for _, m := range matches {
		if m.FinalScore >= threshold {
			filteredMatches = append(filteredMatches, m)
		}
	}
	matches = filteredMatches

	logger.Info(fmt.Sprintf("Service: %d matches above threshold %.2f (from %d candidates)",
		len(matches), threshold, len(candidateIDs)))

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].FinalScore > matches[j].FinalScore
	})

	maxResults := request.GetMaxResults()
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	response := &model.SearchResponse{
		Matches:         matches,
		TotalCandidates: len(candidateIDs),
		ProcessingTime:  time.Since(startTime).Milliseconds(),
	}

	logger.Info(fmt.Sprintf("Service: search complete — %d candidates, %d matches returned in %dms",
		len(candidateIDs), len(matches), response.ProcessingTime))

	return response, nil
}

func (s *IdentityResolutionService) GetPendingReviewTasks(orgHandle string, pageSize int) (*model.ReviewTaskListResponse, error) {
	logger := log.GetLogger()
	logger.Info("Service: fetching pending review tasks", log.String("orgHandle", orgHandle))

	tasks, totalCount, err := irStore.GetPendingReviewTasks(orgHandle, pageSize)
	if err != nil {
		return nil, err
	}

	return &model.ReviewTaskListResponse{
		Pagination: pagination.Pagination{
			Count:    totalCount,
			PageSize: pageSize,
		},
		Tasks: tasks,
	}, nil
}

func (s *IdentityResolutionService) GetPendingReviewTasksByProfile(orgHandle string, profileID string, pageSize int) (*model.ReviewTaskListResponse, error) {
	logger := log.GetLogger()
	logger.Info("Service: fetching review tasks for profile",
		log.String("orgHandle", orgHandle), log.String("profileID", profileID))

	tasks, totalCount, err := irStore.GetPendingReviewTasksByProfile(orgHandle, profileID, pageSize)
	if err != nil {
		return nil, err
	}

	return &model.ReviewTaskListResponse{
		Pagination: pagination.Pagination{
			Count:    totalCount,
			PageSize: pageSize,
		},
		Tasks: tasks,
	}, nil
}

func (s *IdentityResolutionService) ResolveReviewTask(orgHandle string, taskID string, approved bool, resolvedBy string, notes string) error {
	logger := log.GetLogger()

	status := constants.ReviewStatusRejected
	if approved {
		status = constants.ReviewStatusApproved
	}

	logger.Info(fmt.Sprintf("Service: resolving review task %s → %s by %s", taskID, status, resolvedBy))

	task, err := irStore.GetReviewTaskByID(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_NOT_FOUND.Code,
			Message:     errors2.IR_REVIEW_TASK_NOT_FOUND.Message,
			Description: fmt.Sprintf("No review task found with ID %s", taskID),
		}, http.StatusNotFound)
	}
	if task.Status != constants.ReviewStatusPending {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_REVIEW_TASK_RESOLVED.Code,
			Message:     errors2.IR_REVIEW_TASK_RESOLVED.Message,
			Description: fmt.Sprintf("Review task %s has already been %s", taskID, task.Status),
		}, http.StatusConflict)
	}

	if err := irStore.UpdateReviewTaskStatus(taskID, status, resolvedBy, notes); err != nil {
		return err
	}

	if !approved {
		// Store the rejection pair so this combination never resurfaces.
		if err := irStore.InsertRejectionPair(task.OrgHandle, task.SourceProfileID, task.TargetProfileID, resolvedBy); err != nil {
			logger.Warn(fmt.Sprintf("Service: failed to store rejection pair for task %s", taskID), log.Error(err))
		}
		logger.Info(fmt.Sprintf("Service: review task %s rejected — rejection pair stored for '%s' ↔ '%s'",
			taskID, task.SourceProfileID, task.TargetProfileID))
		return nil
	}

	// Cascade cancel: cancel all other PENDING tasks that reference either profile.
	// Only on APPROVED — rejection doesn't change profile data, so other tasks remain valid.
	cancelledSourceIDs, cancelErr := irStore.CancelRelatedReviewTasks(taskID, task.SourceProfileID, task.TargetProfileID, "system")
	if cancelErr != nil {
		logger.Warn(fmt.Sprintf("Service: cascade cancel failed for task %s", taskID), log.Error(cancelErr))
		// Non-fatal — the merge itself will still proceed.
	}

	// Re-enqueue cancelled source profiles for re-evaluation.
	// The UnificationQueue processes them serially — no worker storm.
	for _, srcID := range cancelledSourceIDs {
		p, loadErr := profileStore.GetProfile(srcID)
		if loadErr != nil || p == nil {
			logger.Warn(fmt.Sprintf("Service: skipping re-evaluation for '%s' — profile not found or error", srcID))
			continue
		}
		// Skip if the profile is already a child (merged into another profile).
		if p.ProfileStatus != nil && p.ProfileStatus.ReferenceProfileId != "" {
			logger.Info(fmt.Sprintf("Service: skipping re-evaluation for '%s' — already merged (child of '%s')",
				srcID, p.ProfileStatus.ReferenceProfileId))
			continue
		}
		logger.Info(fmt.Sprintf("Service: re-enqueuing profile '%s' for re-evaluation after cascade cancel", srcID))
		workers.EnqueueProfileForProcessing(*p)
	}

	logger.Info(fmt.Sprintf("Service: review task %s approved — merging profiles '%s' → '%s'",
		taskID, task.SourceProfileID, task.TargetProfileID))

	source, err := profileStore.GetProfile(task.SourceProfileID)
	if err != nil {
		logger.Error("Service: failed to load source profile for review merge", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_MERGE_FAILED.Code,
			Message:     errors2.IR_MERGE_FAILED.Message,
			Description: fmt.Sprintf("Failed to load source profile: %s", task.SourceProfileID),
		}, err)
	}
	if source == nil {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Source profile %s no longer exists", task.SourceProfileID),
		}, http.StatusNotFound)
	}

	target, err := profileStore.GetProfile(task.TargetProfileID)
	if err != nil {
		logger.Error("Service: failed to load target profile for review merge", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_MERGE_FAILED.Code,
			Message:     errors2.IR_MERGE_FAILED.Message,
			Description: fmt.Sprintf("Failed to load target profile: %s", task.TargetProfileID),
		}, err)
	}
	if target == nil {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Target profile %s no longer exists", task.TargetProfileID),
		}, http.StatusNotFound)
	}

	// Redirect to master if either profile is a child (may have been merged since the task was created).
	source, err = redirectToMasterIfChild(source, logger)
	if err != nil {
		return err
	}
	target, err = redirectToMasterIfChild(target, logger)
	if err != nil {
		return err
	}

	// After redirect both might now point to the same master — already merged.
	if source.ProfileId == target.ProfileId {
		logger.Info(fmt.Sprintf("Service: review task %s — source and target resolve to same profile '%s', skipping",
			taskID, source.ProfileId))
		return nil
	}

	primary, secondary := irWorker.DetermineMergeOrder(source, target)

	if source.UserId != "" && target.UserId != "" && source.UserId != target.UserId {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_CANNOT_MERGE.Code,
			Message:     errors2.IR_CANNOT_MERGE.Message,
			Description: "Two permanent profiles with different user IDs cannot be merged.",
		}, http.StatusConflict)
	}

	schemaRules, err := schemaStore.GetProfileSchemaAttributesForOrg(orgHandle)
	if err != nil {
		logger.Error("Service: failed to load schema rules for review merge", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_MERGE_FAILED.Code,
			Message:     errors2.IR_MERGE_FAILED.Message,
			Description: "Failed to load profile schema rules.",
		}, err)
	}

	bothPerm := source.UserId != "" && target.UserId != ""
	bothTemp := source.UserId == "" && target.UserId == ""
	sameType := bothPerm || bothTemp

	primaryRefs, _ := profileStore.FetchReferencedProfiles(primary.ProfileId)
	primaryIsMaster := primary.ProfileStatus != nil &&
		primary.ProfileStatus.IsReferenceProfile &&
		len(primaryRefs) > 0

	mixedType := !sameType
	mergeReason := constants.MergeReasonReviewMerge
	mergeType := constants.MergeTypeReviewMerge
	mergedByVal := resolvedBy

	// survivingProfileID tracks the profile whose data changed after the merge.
	// It must be re-enqueued for re-evaluation with its updated data.
	var survivingProfileID string

	// PATH A: Primary has NO existing children
	if !primaryIsMaster {
		if sameType {
			logger.Info(fmt.Sprintf("Service: same-type review merge — creating new master for '%s' + '%s'",
				source.ProfileId, target.ProfileId))
			masterID, mergeErr := irWorker.CreateMasterProfile(target, source, orgHandle, task.MatchScore,
				schemaRules, mergeReason, mergedByVal)
			if mergeErr != nil {
				return mergeErr
			}
			survivingProfileID = masterID
			logger.Info(fmt.Sprintf("Service: review merge complete — new master '%s' for '%s' + '%s' (task %s)",
				masterID, source.ProfileId, target.ProfileId, taskID))
		} else {
			// Mixed-type, no children → stitch temp into perm.
			// source is the incoming profile (data winner).
			logger.Info(fmt.Sprintf("Service: stitching temp '%s' into perm '%s' (review merge)",
				secondary.ProfileId, primary.ProfileId))
			if mergeErr := irWorker.MergeIntoExisting(secondary, primary, source, orgHandle, task.MatchScore,
				schemaRules, mergeReason, mergeType, mergedByVal); mergeErr != nil {
				return mergeErr
			}
			survivingProfileID = primary.ProfileId
			logger.Info(fmt.Sprintf("Service: review merge complete — '%s' merged into '%s' (task %s)",
				secondary.ProfileId, primary.ProfileId, taskID))
		}
	} else if mixedType && primary.UserId == "" {
		// PATH B: Primary IS an existing temp master — perm takes over.
		logger.Info(fmt.Sprintf("Service: perm '%s' takes over temp master '%s' (review merge — transferring children)",
			secondary.ProfileId, primary.ProfileId))
		if mergeErr := irWorker.MergeWithChildTransfer(secondary, primary, primaryRefs, orgHandle, task.MatchScore,
			schemaRules, mergeReason, mergeType, mergedByVal); mergeErr != nil {
			return mergeErr
		}
		survivingProfileID = secondary.ProfileId
		logger.Info(fmt.Sprintf("Service: review merge complete — perm '%s' took over temp master '%s' (task %s)",
			secondary.ProfileId, primary.ProfileId, taskID))
	} else {
		// All other cases: merge secondary into existing primary master.
		logger.Info(fmt.Sprintf("Service: merging '%s' into existing master '%s' (review merge)",
			secondary.ProfileId, primary.ProfileId))
		if mergeErr := irWorker.MergeIntoExisting(secondary, primary, source, orgHandle, task.MatchScore,
			schemaRules, mergeReason, mergeType, mergedByVal); mergeErr != nil {
			return mergeErr
		}
		survivingProfileID = primary.ProfileId
		logger.Info(fmt.Sprintf("Service: review merge complete — '%s' merged into '%s' (task %s)",
			secondary.ProfileId, primary.ProfileId, taskID))
	}

	// Re-enqueue the surviving profile — its data changed after the merge,
	// so it needs re-evaluation against other profiles. Cancelled bystander
	// source profiles were already re-enqueued above via cascade cancel.
	if survivingProfileID != "" {
		sp, loadErr := profileStore.GetProfile(survivingProfileID)
		if loadErr != nil || sp == nil {
			logger.Warn(fmt.Sprintf("Service: could not re-enqueue surviving profile '%s'", survivingProfileID))
		} else {
			logger.Info(fmt.Sprintf("Service: re-enqueuing surviving profile '%s' for re-evaluation after review merge", survivingProfileID))
			workers.EnqueueProfileForProcessing(*sp)
		}
	}

	return nil
}

func (s *IdentityResolutionService) MergeProfiles(orgHandle string, request *model.MergeRequest) (*model.MergeResponse, error) {
	logger := log.GetLogger()

	if request.NewProfileID == request.CandidateID {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_CANNOT_MERGE.Code,
			Message:     errors2.IR_CANNOT_MERGE.Message,
			Description: "new_profile_id and candidate_id must be different.",
		}, http.StatusBadRequest)
	}

	newProfileID := request.NewProfileID

	// Load both profiles
	candidate, err := profileStore.GetProfile(request.CandidateID)
	if err != nil {
		logger.Error("Service: failed to load candidate profile", log.Error(err))
		return nil, err
	}
	if candidate == nil {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Candidate profile not found: %s", request.CandidateID),
		}, http.StatusNotFound)
	}
	if candidate.OrgHandle != orgHandle {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.FORBIDDEN.Code,
			Message:     errors2.FORBIDDEN.Message,
			Description: "Candidate profile does not belong to the requesting organization.",
		}, http.StatusForbidden)
	}

	newProfile, err := profileStore.GetProfile(newProfileID)
	if err != nil {
		logger.Error("Service: failed to load new profile", log.Error(err))
		return nil, err
	}
	if newProfile == nil {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("New profile not found: %s", newProfileID),
		}, http.StatusNotFound)
	}
	if newProfile.OrgHandle != orgHandle {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.FORBIDDEN.Code,
			Message:     errors2.FORBIDDEN.Message,
			Description: "New profile does not belong to the requesting organization.",
		}, http.StatusForbidden)
	}

	logger.Info("Service: merging profiles",
		log.String("candidate", candidate.ProfileId),
		log.String("newProfile", newProfile.ProfileId))

	candidate, err = redirectToMasterIfChild(candidate, logger)
	if err != nil {
		return nil, err
	}
	newProfile, err = redirectToMasterIfChild(newProfile, logger)
	if err != nil {
		return nil, err
	}

	// After redirect both might now point to the same master.
	if candidate.ProfileId == newProfile.ProfileId {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_CANNOT_MERGE.Code,
			Message:     errors2.IR_CANNOT_MERGE.Message,
			Description: "Both profiles resolve to the same master profile — they are already merged.",
		}, http.StatusConflict)
	}

	if candidate.UserId != "" && newProfile.UserId != "" && candidate.UserId != newProfile.UserId {
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_CANNOT_MERGE.Code,
			Message:     errors2.IR_CANNOT_MERGE.Message,
			Description: "Two permanent profiles with different user IDs cannot be merged.",
		}, http.StatusConflict)
	}

	schemaRules, err := schemaStore.GetProfileSchemaAttributesForOrg(orgHandle)
	if err != nil {
		logger.Error("Service: failed to load schema rules", log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_MERGE_FAILED.Code,
			Message:     errors2.IR_MERGE_FAILED.Message,
			Description: "Failed to load profile schema rules.",
		}, err)
	}

	// Structural merge order: perm always becomes primary (keeps its ID).
	primary, secondary := irWorker.DetermineMergeOrder(newProfile, candidate)

	logger.Info(fmt.Sprintf("Service: merge order — primary '%s' (user=%s), secondary '%s' (user=%s), data winner '%s'",
		primary.ProfileId, primary.UserId, secondary.ProfileId, secondary.UserId, newProfile.ProfileId))

	bothPerm := primary.UserId != "" && secondary.UserId != ""
	bothTemp := primary.UserId == "" && secondary.UserId == ""
	sameType := bothPerm || bothTemp
	mixedType := !sameType

	primaryRefs, _ := profileStore.FetchReferencedProfiles(primary.ProfileId)
	primaryIsMaster := primary.ProfileStatus != nil &&
		primary.ProfileStatus.IsReferenceProfile &&
		len(primaryRefs) > 0

	mergeReason := constants.MergeReasonManualMerge
	mergeType := constants.MergeTypeManualMerge
	mergedBy := ""

	// PATH A: Primary has NO existing children
	if !primaryIsMaster {
		if sameType {
			logger.Info(fmt.Sprintf("Service: same-type manual merge — creating new master for '%s' + '%s'",
				primary.ProfileId, secondary.ProfileId))

			masterID, err := irWorker.CreateMasterProfile(candidate, newProfile, orgHandle, 0,
				schemaRules, mergeReason, mergedBy)
			if err != nil {
				return nil, err
			}

			logger.Info(fmt.Sprintf("Service: manual merge complete — new master '%s' created for '%s' + '%s'",
				masterID, primary.ProfileId, secondary.ProfileId))
			return &model.MergeResponse{
				Status:          "success",
				Message:         fmt.Sprintf("Profiles %s and %s merged into new master %s", primary.ProfileId, secondary.ProfileId, masterID),
				MergedProfileID: masterID,
			}, nil
		}

		// Mixed-type, no children → stitch temp into perm.
		logger.Info(fmt.Sprintf("Service: stitching temp '%s' into perm '%s' (manual merge)",
			secondary.ProfileId, primary.ProfileId))
		if err := irWorker.MergeIntoExisting(secondary, primary, newProfile, orgHandle, 0,
			schemaRules, mergeReason, mergeType, mergedBy); err != nil {
			return nil, err
		}
		logger.Info(fmt.Sprintf("Service: merge complete — '%s' merged into '%s'",
			secondary.ProfileId, primary.ProfileId))
		return &model.MergeResponse{
			Status:          "success",
			Message:         fmt.Sprintf("Profile %s merged into %s", secondary.ProfileId, primary.ProfileId),
			MergedProfileID: primary.ProfileId,
		}, nil
	}

	// PATH B: Primary IS an existing master
	if mixedType && primary.UserId == "" {
		// Primary master is temp, secondary is perm → perm takes over, transfer children.
		logger.Info(fmt.Sprintf("Service: perm '%s' takes over temp master '%s' (manual merge — transferring children)",
			secondary.ProfileId, primary.ProfileId))
		if err := irWorker.MergeWithChildTransfer(secondary, primary, primaryRefs, orgHandle, 0,
			schemaRules, mergeReason, mergeType, mergedBy); err != nil {
			return nil, err
		}
		logger.Info(fmt.Sprintf("Service: merge complete — perm '%s' took over temp master '%s'",
			secondary.ProfileId, primary.ProfileId))
		return &model.MergeResponse{
			Status:          "success",
			Message:         fmt.Sprintf("Profile %s took over master %s", secondary.ProfileId, primary.ProfileId),
			MergedProfileID: secondary.ProfileId,
		}, nil
	}

	// All other cases: merge secondary into existing primary master.
	logger.Info(fmt.Sprintf("Service: merging '%s' into existing master '%s' (manual merge)",
		secondary.ProfileId, primary.ProfileId))
	if err := irWorker.MergeIntoExisting(secondary, primary, newProfile, orgHandle, 0,
		schemaRules, mergeReason, mergeType, mergedBy); err != nil {
		return nil, err
	}
	logger.Info(fmt.Sprintf("Service: merge complete — '%s' merged into '%s'",
		secondary.ProfileId, primary.ProfileId))
	return &model.MergeResponse{
		Status:          "success",
		Message:         fmt.Sprintf("Profile %s merged into %s", secondary.ProfileId, primary.ProfileId),
		MergedProfileID: primary.ProfileId,
	}, nil
}

func redirectToMasterIfChild(p *profileModel.Profile, logger *log.Logger) (*profileModel.Profile, error) {
	if p.ProfileStatus == nil || p.ProfileStatus.ReferenceProfileId == "" {
		return p, nil
	}
	masterID := p.ProfileStatus.ReferenceProfileId
	logger.Info(fmt.Sprintf("Service: profile '%s' is a child of master '%s' — redirecting merge to master",
		p.ProfileId, masterID))

	master, err := profileStore.GetProfile(masterID)
	if err != nil || master == nil {
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_MERGE_FAILED.Code,
			Message:     errors2.IR_MERGE_FAILED.Message,
			Description: fmt.Sprintf("Failed to load master profile '%s' for redirect", masterID),
		}, err)
	}
	return master, nil
}

func loadEnrichedRules(orgHandle string) ([]*model.EnrichedRule, error) {
	logger := log.GetLogger()
	logger.Debug("Service: loading unification rules", log.String("orgHandle", orgHandle))

	rawRules, err := urStore.GetUnificationRules(orgHandle)
	if err != nil {
		logger.Error("Service: failed to load unification rules", log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_SEARCH_FAILED.Code,
			Message:     errors2.IR_SEARCH_FAILED.Message,
			Description: fmt.Sprintf("Failed to load unification rules for org: %s", orgHandle),
		}, err)
	}

	enriched := model.EnrichRulesWithSampling(rawRules, orgHandle, irStore.SampleAttributeValues, buildSchemaLookup)
	logger.Info(fmt.Sprintf("Service: enriched %d active rules from %d total", len(enriched), len(rawRules)))

	return enriched, nil
}

func buildSchemaLookup(orgHandle string) (map[string]string, error) {
	attrs, err := schemaStore.GetProfileSchemaAttributesForOrg(orgHandle)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(attrs))
	for _, a := range attrs {
		result[a.AttributeId] = a.ValueType
	}
	return result, nil
}
