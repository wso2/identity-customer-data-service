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

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	irModel "github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/pagination"
	"github.com/wso2/identity-customer-data-service/internal/system/security"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
)

type IdentityResolutionHandler struct{}

func NewIdentityResolutionHandler() *IdentityResolutionHandler {
	return &IdentityResolutionHandler{}
}

func (h *IdentityResolutionHandler) Search(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger()
	logger.Info("Handler: identity resolution search request received")

	// Auth check
	err := security.AuthnAndAuthz(r, "identity_resolution:search")
	if err != nil {
		logger.Warn("Handler: authentication/authorization failed", log.Error(err))
		utils.HandleError(w, err)
		return
	}

	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if orgHandle == "" {
		logger.Warn("Handler: missing org handle in request")
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "Organization handle is required.",
		}, http.StatusBadRequest))
		return
	}

	var request irModel.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Warn("Handler: invalid request body", log.Error(err))
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Failed to parse search request: %v", err),
		}, http.StatusBadRequest))
		return
	}

	if len(request.IdentityAttributes) == 0 && len(request.Traits) == 0 {
		logger.Warn("Handler: no identity_attributes or traits in search request")
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "At least one identity_attribute or trait is required for search.",
		}, http.StatusBadRequest))
		return
	}

	svc := provider.NewIdentityResolutionProvider().GetIdentityResolutionService()
	response, err := svc.Search(orgHandle, &request)
	if err != nil {
		logger.Error("Handler: search failed", log.Error(err))
		utils.HandleError(w, err)
		return
	}

	logger.Info(fmt.Sprintf("Handler: search completed — %d matches found", len(response.Matches)))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *IdentityResolutionHandler) GetReviewTasks(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger()
	logger.Info("Handler: get review tasks request received")

	err := security.AuthnAndAuthz(r, "identity_resolution:review")
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if orgHandle == "" {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "Organization handle is required.",
		}, http.StatusBadRequest))
		return
	}

	pageSize, parseErr := pagination.ParsePageSize(r)
	if parseErr != nil {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: parseErr.Error(),
		}, http.StatusBadRequest))
		return
	}

	svc := provider.NewIdentityResolutionProvider().GetIdentityResolutionService()

	profileID := r.URL.Query().Get("profile_id")

	var response *irModel.ReviewTaskListResponse
	if profileID != "" {
		response, err = svc.GetPendingReviewTasksByProfile(orgHandle, profileID, pageSize)
	} else {
		response, err = svc.GetPendingReviewTasks(orgHandle, pageSize)
	}
	if err != nil {
		logger.Error("Handler: failed to fetch review tasks", log.Error(err))
		utils.HandleError(w, err)
		return
	}

	logger.Info(fmt.Sprintf("Handler: returning %d review tasks (total %d)", len(response.Tasks), response.Pagination.Count))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (h *IdentityResolutionHandler) ResolveReviewTask(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger()
	logger.Info("Handler: resolve review task request received")

	err := security.AuthnAndAuthz(r, "identity_resolution:review")
	if err != nil {
		utils.HandleError(w, err)
		return
	}
	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if orgHandle == "" {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "Organization handle is required.",
		}, http.StatusBadRequest))
		return
	}
	taskID := r.PathValue("taskId")
	if taskID == "" {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "Task ID is required.",
		}, http.StatusBadRequest))
		return
	}

	var resolveReq irModel.ReviewResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&resolveReq); err != nil {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Failed to parse resolve request: %v", err),
		}, http.StatusBadRequest))
		return
	}

	if resolveReq.Decision != constants.ReviewStatusApproved && resolveReq.Decision != constants.ReviewStatusRejected {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Invalid decision '%s'. Must be '%s' or '%s'.",
				resolveReq.Decision, constants.ReviewStatusApproved, constants.ReviewStatusRejected),
		}, http.StatusBadRequest))
		return
	}

	// TODO: Extract actual user from auth token
	resolvedBy := constants.MergeByAdmin
	approved := resolveReq.Decision == constants.ReviewStatusApproved

	svc := provider.NewIdentityResolutionProvider().GetIdentityResolutionService()
	err = svc.ResolveReviewTask(orgHandle, taskID, approved, resolvedBy, resolveReq.Notes)
	if err != nil {
		logger.Error("Handler: failed to resolve review task", log.Error(err))
		utils.HandleError(w, err)
		return
	}

	action := "rejected"
	if approved {
		action = "approved"
	}
	logger.Info(fmt.Sprintf("Handler: review task %s %s by %s", taskID, action, resolvedBy))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Review task %s %s", taskID, action),
	})
}

func (h *IdentityResolutionHandler) MergeProfiles(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger()
	logger.Info("Handler: merge profiles request received")

	err := security.AuthnAndAuthz(r, "identity_resolution:merge")
	if err != nil {
		logger.Warn("Handler: authentication/authorization failed", log.Error(err))
		utils.HandleError(w, err)
		return
	}

	orgHandle := utils.ExtractOrgHandleFromPath(r)
	if orgHandle == "" {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "Organization handle is required.",
		}, http.StatusBadRequest))
		return
	}

	var request irModel.MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logger.Warn("Handler: invalid merge request body", log.Error(err))
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: fmt.Sprintf("Failed to parse merge request: %v", err),
		}, http.StatusBadRequest))
		return
	}

	if request.NewProfileID == "" {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "new_profile_id is required. Use the new_profile_id from the search response.",
		}, http.StatusBadRequest))
		return
	}

	if request.CandidateID == "" {
		utils.HandleError(w, errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.BAD_REQUEST.Code,
			Message:     errors2.BAD_REQUEST.Message,
			Description: "candidate_id is required. Use a candidate_id from the search results.",
		}, http.StatusBadRequest))
		return
	}

	svc := provider.NewIdentityResolutionProvider().GetIdentityResolutionService()
	response, err := svc.MergeProfiles(orgHandle, &request)
	if err != nil {
		logger.Error("Handler: merge failed", log.Error(err))
		utils.HandleError(w, err)
		return
	}

	logger.Info(fmt.Sprintf("Handler: merge completed — %s", response.MergedProfileID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
