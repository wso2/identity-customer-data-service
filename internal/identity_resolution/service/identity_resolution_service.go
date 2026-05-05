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

	if !approved {
		// Store the rejection pair before marking the task rejected.
		if err := irStore.InsertRejectionPair(task.OrgHandle, task.IncomingProfileID, task.CandidateProfileID, resolvedBy); err != nil {
			logger.Error(fmt.Sprintf("Service: failed to store rejection pair for task %s", taskID), log.Error(err))
			return err
		}
		if err := irStore.UpdateReviewTaskStatus(taskID, status, resolvedBy, notes); err != nil {
			return err
		}
		return nil
	}

	// Cascade cancel: cancel all other PENDING tasks that reference either profile.
	// Only on APPROVED — rejection doesn't change profile data, so other tasks remain valid.
	cancelledIncomingIDs, cancelErr := irStore.CancelRelatedReviewTasks(taskID, task.IncomingProfileID, task.CandidateProfileID, constants.CanceledBySystem)
	if cancelErr != nil {
		logger.Warn(fmt.Sprintf("Service: cascade cancel failed for task %s", taskID), log.Error(cancelErr))
		// merge itself will still proceed.
	}

	// Re-enqueue cancelled incoming profiles for re-evaluation.
	for _, incomingID := range cancelledIncomingIDs {
		p, loadErr := profileStore.GetProfile(incomingID)
		if loadErr != nil || p == nil {
			logger.Warn(fmt.Sprintf("Service: skipping re-evaluation for '%s' — profile not found or error", incomingID))
			continue
		}

		if p.ProfileStatus != nil && p.ProfileStatus.ReferenceProfileId != "" {
			masterID := p.ProfileStatus.ReferenceProfileId
			master, err := profileStore.GetProfile(masterID)
			if err == nil && master != nil {
				workers.EnqueueProfileForProcessing(*master)
			}
			continue
		}

		workers.EnqueueProfileForProcessing(*p)
	}

	incomingProfile, err := profileStore.GetProfile(task.IncomingProfileID)
	if err != nil {
		logger.Error("Service: failed to load incoming profile for review merge", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_MERGE_FAILED.Code,
			Message:     errors2.IR_MERGE_FAILED.Message,
			Description: fmt.Sprintf("Failed to load incoming profile: %s", task.IncomingProfileID),
		}, err)
	}
	if incomingProfile == nil {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("incoming profile %s no longer exists", task.IncomingProfileID),
		}, http.StatusNotFound)
	}

	candidate, err := profileStore.GetProfile(task.CandidateProfileID)
	if err != nil {
		logger.Error("Service: failed to load candidate profile for review merge", log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_MERGE_FAILED.Code,
			Message:     errors2.IR_MERGE_FAILED.Message,
			Description: fmt.Sprintf("Failed to load candidate profile: %s", task.CandidateProfileID),
		}, err)
	}
	if candidate == nil {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.PROFILE_NOT_FOUND.Code,
			Message:     errors2.PROFILE_NOT_FOUND.Message,
			Description: fmt.Sprintf("Candidate profile %s no longer exists", task.CandidateProfileID),
		}, http.StatusNotFound)
	}

	// Redirect to master if either profile is a child (may have been merged since the task was created).
	incomingProfile, err = redirectToMasterIfChild(incomingProfile, logger)
	if err != nil {
		return err
	}
	candidate, err = redirectToMasterIfChild(candidate, logger)
	if err != nil {
		return err
	}

	// After redirect both might now point to the same master — already merged.
	if incomingProfile.ProfileId == candidate.ProfileId {
		return nil
	}

	if incomingProfile.UserId != "" && candidate.UserId != "" && incomingProfile.UserId != candidate.UserId {
		return errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.IR_CANNOT_MERGE.Code,
			Message:     errors2.IR_CANNOT_MERGE.Message,
			Description: "Two permanent profiles with different user IDs cannot be merged.",
		}, http.StatusConflict)
	}

	// Run the merge BEFORE updating task status. If MergeMatchedProfiles surfaces
	// an error, the task must stay PENDING so the caller can retry.
	if mergeErr := workers.MergeMatchedProfiles(*candidate, *incomingProfile, constants.MergeReasonReviewMerge); mergeErr != nil {
		logger.Error(fmt.Sprintf("Service: review merge failed for task %s — '%s' and '%s'",
			taskID, incomingProfile.ProfileId, candidate.ProfileId), log.Error(mergeErr))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:    errors2.IR_MERGE_FAILED.Code,
			Message: errors2.IR_MERGE_FAILED.Message,
			Description: fmt.Sprintf("Failed to merge profiles '%s' and '%s' for review task %s",
				incomingProfile.ProfileId, candidate.ProfileId, taskID),
		}, mergeErr)
	}

	// Merge succeeded — commit task status.
	if err := irStore.UpdateReviewTaskStatus(taskID, status, resolvedBy, notes); err != nil {
		logger.Error(fmt.Sprintf("Service: merge succeeded but failed to update task %s status to %s",
			taskID, status), log.Error(err))
		return err
	}

	return nil
}

func redirectToMasterIfChild(p *profileModel.Profile, logger *log.Logger) (*profileModel.Profile, error) {
	if p.ProfileStatus == nil || p.ProfileStatus.ReferenceProfileId == "" {
		return p, nil
	}
	masterID := p.ProfileStatus.ReferenceProfileId

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
