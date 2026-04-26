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

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	irStore "github.com/wso2/identity-customer-data-service/internal/identity_resolution/store"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/pagination"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
)

type IdentityResolutionServiceInterface interface {
	GetPendingReviewTasks(orgHandle string, pageSize int) (*model.ReviewTaskListResponse, error)

	GetPendingReviewTasksByProfile(orgHandle string, profileID string, pageSize int) (*model.ReviewTaskListResponse, error)

	ResolveReviewTask(orgHandle string, taskID string, approved bool, resolvedBy string, notes string) error
}

type IdentityResolutionService struct{}

func GetIdentityResolutionService() IdentityResolutionServiceInterface {
	return &IdentityResolutionService{}
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

	if source.UserId != "" && target.UserId != "" && source.UserId != target.UserId {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_CANNOT_MERGE.Code,
			Message:     errors2.IR_CANNOT_MERGE.Message,
			Description: "Two permanent profiles with different user IDs cannot be merged.",
		}, http.StatusConflict)
	}

	// Use the existing merge function from profile_worker — it handles all
	// merge scenarios (same-type, mixed-type, master with children, etc.).
	workers.MergeMatchedProfiles(*target, *source, constants.MergeReasonReviewMerge)

	logger.Info(fmt.Sprintf("Service: review merge complete for task %s — '%s' and '%s'",
		taskID, source.ProfileId, target.ProfileId))

	return nil
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
