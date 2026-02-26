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

package worker

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	irStore "github.com/wso2/identity-customer-data-service/internal/identity_resolution/store"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaStore "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	urStore "github.com/wso2/identity-customer-data-service/internal/unification_rules/store"
)

func ResolveProfileAsync(profile profileModel.Profile) {
	logger := log.GetLogger()

	logger.Info(fmt.Sprintf("AsyncWorker: starting identity resolution for profile '%s' (org=%s)",
		profile.ProfileId, profile.OrgHandle))

	freshProfile, err := profileStore.GetProfile(profile.ProfileId)
	if err != nil || freshProfile == nil {
		logger.Error(fmt.Sprintf("AsyncWorker: failed to fetch profile '%s', skipping", profile.ProfileId))
		return
	}

	orgHandle := freshProfile.OrgHandle

	rawRules, err := urStore.GetUnificationRules(orgHandle)
	if err != nil {
		logger.Error(fmt.Sprintf("AsyncWorker: failed to load unification rules for org '%s'", orgHandle), log.Error(err))
		return
	}

	rules := model.EnrichRulesWithSampling(rawRules, orgHandle, irStore.SampleAttributeValues, buildSchemaLookup)
	if len(rules) == 0 {
		logger.Info(fmt.Sprintf("AsyncWorker: no active unification rules for org '%s', skipping", orgHandle))
		return
	}

	logger.Info(fmt.Sprintf("AsyncWorker: loaded %d enriched rules", len(rules)))

	flatAttrs := flattenProfile(freshProfile)

	excludeID := freshProfile.ProfileId
	parentID := ""
	if freshProfile.ProfileStatus != nil && freshProfile.ProfileStatus.ReferenceProfileId != "" {
		parentID = freshProfile.ProfileStatus.ReferenceProfileId
	}

	blockingKeys := engine.GenerateProfileBlockingKeys(rules, flatAttrs)

	if len(blockingKeys) > 0 {
		if err := irStore.UpsertBlockingKeys(freshProfile.ProfileId, orgHandle, blockingKeys); err != nil {
			logger.Warn(fmt.Sprintf("AsyncWorker: failed to index profile '%s' in blocking_keys",
				freshProfile.ProfileId), log.Error(err))
		} else {
			logger.Info(fmt.Sprintf("AsyncWorker: indexed %d blocking keys for profile '%s'",
				len(blockingKeys), freshProfile.ProfileId))
		}
	}

	candidateIDs := engine.FindCandidatesByIndex(blockingKeys, orgHandle, excludeID, irStore.FindCandidateIDsByKeys)

	if parentID != "" {
		filtered := make([]string, 0, len(candidateIDs))
		for _, id := range candidateIDs {
			if id != parentID {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) < len(candidateIDs) {
			logger.Info(fmt.Sprintf("AsyncWorker: excluded parent profile '%s' from candidates", parentID))
		}
		candidateIDs = filtered
	}

	if len(candidateIDs) == 0 {
		logger.Info(fmt.Sprintf("AsyncWorker: no candidates found for profile '%s'", freshProfile.ProfileId))
		return
	}

	candidateProfiles, err := irStore.GetProfilesByIDs(candidateIDs)
	if err != nil {
		logger.Error(fmt.Sprintf("AsyncWorker: failed to load candidate profiles for org '%s'", orgHandle), log.Error(err))
		return
	}

	profileMap := make(map[string]*model.ProfileData, len(candidateProfiles))
	for i := range candidateProfiles {
		profileMap[candidateProfiles[i].ProfileID] = &candidateProfiles[i]
	}

	// Resolve child candidates to their master profiles.
	// If a candidate is a child (merged into a master), replace it with the master ID.
	// This prevents creating redundant review tasks for both a master and its child
	// against the same target profile — they share the same data.
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
				// Skip if the master is the incoming profile itself.
				if masterID == freshProfile.ProfileId {
					logger.Info(fmt.Sprintf("AsyncWorker: child '%s' belongs to incoming profile '%s' — skipping",
						cid, freshProfile.ProfileId))
					continue
				}
				// Skip if the master is the parent we already excluded.
				if masterID == parentID {
					logger.Info(fmt.Sprintf("AsyncWorker: child '%s' belongs to excluded parent '%s' — skipping",
						cid, parentID))
					continue
				}
				resolvedID = masterID
				// Load master into profileMap if not already there.
				if _, ok := profileMap[masterID]; !ok {
					masterProfiles, loadErr := irStore.GetProfilesByIDs([]string{masterID})
					if loadErr != nil || len(masterProfiles) == 0 {
						logger.Warn(fmt.Sprintf("AsyncWorker: could not load master '%s' for child '%s', skipping child",
							masterID, cid))
						continue
					}
					profileMap[masterID] = &masterProfiles[0]
				}
				logger.Info(fmt.Sprintf("AsyncWorker: resolved child candidate '%s' → master '%s'",
					cid, masterID))
			}
			if !seen[resolvedID] {
				seen[resolvedID] = true
				resolvedIDs = append(resolvedIDs, resolvedID)
			}
		}
		candidateIDs = resolvedIDs
	}

	if len(candidateIDs) == 0 {
		logger.Info(fmt.Sprintf("AsyncWorker: no candidates left after resolving children for profile '%s'", freshProfile.ProfileId))
		return
	}

	inputProfileType := model.DetermineProfileType(flatAttrs)
	thresholds := model.LoadThresholds(orgHandle)

	type scoredCandidate struct {
		id        string
		score     float64
		breakdown map[string]float64
	}
	var scored []scoredCandidate

	for _, candidateID := range candidateIDs {
		candidate, exists := profileMap[candidateID]
		if !exists {
			continue
		}

		candidateType := candidate.GetProfileType()
		mode := model.DetermineMode(inputProfileType, candidateType, thresholds.SmartResolutionEnabled)

		finalScore, breakdown := engine.ScoreCandidate(flatAttrs, candidate, rules, mode, thresholds.AutoMerge)

		if finalScore >= thresholds.ManualReview {
			scored = append(scored, scoredCandidate{
				id:        candidateID,
				score:     finalScore,
				breakdown: breakdown,
			})
		}
	}

	if len(scored) == 0 {
		logger.Info(fmt.Sprintf("AsyncWorker: no candidates above review threshold for profile '%s'", freshProfile.ProfileId))
		return
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	logger.Info(fmt.Sprintf("AsyncWorker: %d candidates above threshold (best=%.4f) for profile '%s'",
		len(scored), scored[0].score, freshProfile.ProfileId))

	// Filter out candidates that have been explicitly rejected against this profile.
	rejectedIDs, _ := irStore.GetRejectedProfileIDs(orgHandle, freshProfile.ProfileId)
	if len(rejectedIDs) > 0 {
		var filtered []scoredCandidate
		for _, sc := range scored {
			if rejectedIDs[sc.id] {
				logger.Info(fmt.Sprintf("AsyncWorker: skipping rejected pair '%s' ↔ '%s'",
					freshProfile.ProfileId, sc.id))
				continue
			}
			filtered = append(filtered, sc)
		}
		scored = filtered
		if len(scored) == 0 {
			logger.Info(fmt.Sprintf("AsyncWorker: all candidates rejected for profile '%s'", freshProfile.ProfileId))
			return
		}
	}

	// Filter out unmergeable pairs: two permanent profiles with different user IDs can never be merged,
	// so creating review tasks for them is pointless.
	if freshProfile.UserId != "" {
		var mergeable []scoredCandidate
		for _, sc := range scored {
			candidate := profileMap[sc.id]
			if candidate.UserID != "" && candidate.UserID != freshProfile.UserId {
				logger.Info(fmt.Sprintf("AsyncWorker: skipping unmergeable pair — '%s' (user=%s) ↔ '%s' (user=%s): different permanent users",
					freshProfile.ProfileId, freshProfile.UserId, sc.id, candidate.UserID))
				continue
			}
			mergeable = append(mergeable, sc)
		}
		scored = mergeable
		if len(scored) == 0 {
			logger.Info(fmt.Sprintf("AsyncWorker: no mergeable candidates left for profile '%s' after filtering unmergeable pairs",
				freshProfile.ProfileId))
			return
		}
	}

	merged := false
	for _, sc := range scored {
		decision := model.Decide(sc.score, thresholds)

		switch decision {
		case constants.DecisionAutoMerge:
			if !merged {
				logger.Info(fmt.Sprintf("AsyncWorker: AUTO_MERGE (best-fit) — profile '%s' matches '%s' (score=%.4f)",
					freshProfile.ProfileId, sc.id, sc.score))

				if err := autoMerge(freshProfile, sc.id, orgHandle, sc.score); err != nil {
					logger.Error(fmt.Sprintf("AsyncWorker: auto-merge failed for '%s' → '%s'",
						freshProfile.ProfileId, sc.id), log.Error(err))
					insertReviewTaskSafe(orgHandle, freshProfile.ProfileId, sc.id, sc.score, sc.breakdown)
				} else {
					merged = true
					logger.Info(fmt.Sprintf("AsyncWorker: profile '%s' consumed by merge — skipping remaining candidates",
						freshProfile.ProfileId))
				}
			}

		case constants.DecisionManualReview:
			if !merged {
				logger.Info(fmt.Sprintf("AsyncWorker: MANUAL_REVIEW — profile '%s' matches '%s' (score=%.4f)",
					freshProfile.ProfileId, sc.id, sc.score))
				insertReviewTaskSafe(orgHandle, freshProfile.ProfileId, sc.id, sc.score, sc.breakdown)
			}
		}

		if merged {
			break
		}
	}

	logger.Info(fmt.Sprintf("AsyncWorker: resolution complete for profile '%s' (merged=%v)",
		freshProfile.ProfileId, merged))
}

func autoMerge(incoming *profileModel.Profile, matchedID string, orgHandle string, score float64) error {
	logger := log.GetLogger()

	matched, err := profileStore.GetProfile(matchedID)
	if err != nil || matched == nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: fmt.Sprintf("Failed to load matched profile: %s", matchedID),
		}, err)
	}

	if matched.ProfileStatus != nil && matched.ProfileStatus.ReferenceProfileId != "" {
		masterID := matched.ProfileStatus.ReferenceProfileId

		// If the incoming profile is a sibling (same parent), they are already unified — skip.
		if incoming.ProfileStatus != nil && incoming.ProfileStatus.ReferenceProfileId == masterID {
			logger.Info(fmt.Sprintf("AsyncWorker: skipping merge — '%s' and '%s' are already siblings under master '%s'",
				incoming.ProfileId, matched.ProfileId, masterID))
			return nil
		}

		logger.Info(fmt.Sprintf("AsyncWorker: candidate '%s' is a child of master '%s' — redirecting merge to master",
			matched.ProfileId, masterID))

		master, loadErr := profileStore.GetProfile(masterID)
		if loadErr != nil || master == nil {
			logger.Warn(fmt.Sprintf("AsyncWorker: could not load master '%s' for child '%s', falling back to review task",
				masterID, matched.ProfileId))
			return errors2.NewServerError(errors2.ErrorMessage{
				Code:        errors2.IR_WORKER_FAILED.Code,
				Message:     errors2.IR_WORKER_FAILED.Message,
				Description: fmt.Sprintf("Failed to load master profile '%s' for redirect", masterID),
			}, loadErr)
		}

		matched = master
		logger.Info(fmt.Sprintf("AsyncWorker: merge target redirected to master '%s'", matched.ProfileId))
	}

	hasUserIDIncoming := incoming.UserId != ""
	hasUserIDMatched := matched.UserId != ""

	// Guard: refuse to merge two permanent profiles with different user_ids.
	if hasUserIDIncoming && hasUserIDMatched && incoming.UserId != matched.UserId {
		logger.Info(fmt.Sprintf("AsyncWorker: refusing auto-merge — different user_ids ('%s' vs '%s')",
			incoming.UserId, matched.UserId))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:    errors2.IR_WORKER_FAILED.Code,
			Message: errors2.IR_WORKER_FAILED.Message,
			Description: fmt.Sprintf("Cannot auto-merge two permanent profiles with different user_ids ('%s' vs '%s')",
				incoming.UserId, matched.UserId),
		}, fmt.Errorf("different user_ids: %s vs %s", incoming.UserId, matched.UserId))
	}

	schemaRules, err := schemaStore.GetProfileSchemaAttributesForOrg(orgHandle)
	if err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: "Failed to load schema rules for merge.",
		}, err)
	}

	// Fetch existing references to decide the path.
	matchedRefs, _ := profileStore.FetchReferencedProfiles(matched.ProfileId)
	matchedIsMaster := matched.ProfileStatus != nil &&
		matched.ProfileStatus.IsReferenceProfile &&
		len(matchedRefs) > 0

	mixedType := hasUserIDIncoming != hasUserIDMatched // perm-temp or temp-perm

	// PATH A: Matched profile has NO existing children
	if !matchedIsMaster {

		// A1: perm-temp or temp-perm → stitch temp into perm.
		if mixedType {
			primary, secondary := DetermineMergeOrder(incoming, matched)
			logger.Info(fmt.Sprintf("AsyncWorker: stitching temp '%s' into perm '%s'",
				secondary.ProfileId, primary.ProfileId))
			return mergeIntoExisting(secondary, primary, incoming, orgHandle, score,
				schemaRules, constants.MergeReasonAutoMerge, constants.MergeTypeAutoMerge, constants.MergedBySystem)
		}

		// A2: same-type → create a new master profile.
		userId := ""
		if hasUserIDIncoming && hasUserIDMatched {
			userId = incoming.UserId // already validated they are equal
			logger.Info(fmt.Sprintf("AsyncWorker: both permanent (same user_id) — creating new master for '%s' + '%s'",
				incoming.ProfileId, matched.ProfileId))
		} else {
			logger.Info(fmt.Sprintf("AsyncWorker: both temporary — creating new master for '%s' + '%s'",
				incoming.ProfileId, matched.ProfileId))
		}
		_, err := createMasterProfile(matched, incoming, orgHandle, score,
			schemaRules, constants.MergeReasonAutoMerge, constants.MergedBySystem, userId)
		return err
	}

	// PATH B: Matched profile IS an existing master (has children)

	// B1: perm-temp or temp-perm.
	if mixedType {
		if hasUserIDMatched {
			// Matched master is perm → incoming temp merges into it.
			logger.Info(fmt.Sprintf("AsyncWorker: merging temp '%s' into existing perm master '%s'",
				incoming.ProfileId, matched.ProfileId))
			return mergeIntoExisting(incoming, matched, incoming, orgHandle, score,
				schemaRules, constants.MergeReasonAutoMerge, constants.MergeTypeAutoMerge, constants.MergedBySystem)
		}
		// Matched master is temp, incoming is perm → perm takes over, transfer children.
		logger.Info(fmt.Sprintf("AsyncWorker: perm '%s' takes over temp master '%s' — transferring children",
			incoming.ProfileId, matched.ProfileId))
		return MergeWithChildTransfer(incoming, matched, matchedRefs, orgHandle, score,
			schemaRules, constants.MergeReasonAutoMerge, constants.MergeTypeAutoMerge, constants.MergedBySystem)
	}

	// B2: same-type → merge new profile into the existing master.
	if hasUserIDIncoming && hasUserIDMatched && incoming.UserId != matched.UserId {
		// Should not reach here due to guard above, but be safe.
		logger.Info("AsyncWorker: refusing merge — different userIds on master path")
		return fmt.Errorf("cannot merge permanent profiles with different userIds")
	}

	logger.Info(fmt.Sprintf("AsyncWorker: merging same-type '%s' into existing master '%s'",
		incoming.ProfileId, matched.ProfileId))
	return mergeIntoExisting(incoming, matched, incoming, orgHandle, score,
		schemaRules, constants.MergeReasonAutoMerge, constants.MergeTypeAutoMerge, constants.MergedBySystem)
}

// CreateMasterProfile creates a new master profile with both profiles as children.
func CreateMasterProfile(existing, incoming *profileModel.Profile, orgHandle string, score float64,
	schemaRules []schemaModel.ProfileSchemaAttribute, mergeReason, mergedBy string) (string, error) {
	userId := ""
	if existing.UserId != "" {
		userId = existing.UserId
	}
	if incoming.UserId != "" {
		userId = incoming.UserId
	}
	return createMasterProfile(existing, incoming, orgHandle, score, schemaRules, mergeReason, mergedBy, userId)
}

func createMasterProfile(existing, incoming *profileModel.Profile, orgHandle string, score float64,
	schemaRules []schemaModel.ProfileSchemaAttribute, mergeReason, mergedBy, userId string) (string, error) {

	logger := log.GetLogger()

	masterID := uuid.New().String()
	now := time.Now().UTC()

	childProfile1 := profileModel.Reference{
		ProfileId: existing.ProfileId,
		Reason:    mergeReason,
	}
	childProfile2 := profileModel.Reference{
		ProfileId: incoming.ProfileId,
		Reason:    mergeReason,
	}

	masterProfile := profileModel.Profile{
		ProfileId: masterID,
		UserId:    userId,
		OrgHandle: orgHandle,
		CreatedAt: now,
		UpdatedAt: now,
		Location:  utils.BuildProfileLocation(orgHandle, masterID),
		ProfileStatus: &profileModel.ProfileStatus{
			IsReferenceProfile: true,
			ListProfile:        false,
			References:         []profileModel.Reference{childProfile1, childProfile2},
		},
	}

	if err := profileStore.InsertProfile(masterProfile); err != nil {
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: fmt.Sprintf("Failed to insert new master profile: %s", masterID),
		}, err)
	}

	logger.Info(fmt.Sprintf("Worker: created new master profile '%s' for org '%s'", masterID, orgHandle))

	children := []profileModel.Reference{childProfile1, childProfile2}
	if err := profileStore.UpdateProfileReferences(masterProfile, children); err != nil {
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: fmt.Sprintf("Failed to update profile references for master '%s'", masterID),
		}, err)
	}

	merged := workers.MergeProfiles(*existing, *incoming, schemaRules)

	// Persist merged data section by section — matching the old proven pattern.
	if err := persistMergedDataSections(masterID, merged, logger); err != nil {
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: fmt.Sprintf("Failed to persist merged data to master '%s'", masterID),
		}, err)
	}

	_ = irStore.InsertMergeAuditLog(model.MergeAuditEntry{
		OrgHandle:          orgHandle,
		PrimaryProfileID:   masterID,
		SecondaryProfileID: fmt.Sprintf("%s,%s", existing.ProfileId, incoming.ProfileId),
		MergeType:          constants.MergeTypeNewMaster,
		MatchScore:         score,
		MergedBy:           mergedBy,
	})

	if err := irStore.DeleteBlockingKeys(existing.ProfileId); err != nil {
		logger.Warn(fmt.Sprintf("Worker: failed to delete blocking keys for '%s'", existing.ProfileId),
			log.Error(err))
	}
	ReindexAfterMerge(masterID, existing.ProfileId, orgHandle, merged)

	logger.Info(fmt.Sprintf("Worker: new master '%s' created — children: ['%s', '%s']",
		masterID, existing.ProfileId, incoming.ProfileId))
	return masterID, nil
}

func MergeIntoExisting(absorbed, primary, incoming *profileModel.Profile, orgHandle string, score float64,
	schemaRules []schemaModel.ProfileSchemaAttribute, mergeReason, mergeType, mergedBy string) error {
	return mergeIntoExisting(absorbed, primary, incoming, orgHandle, score, schemaRules, mergeReason, mergeType, mergedBy)
}

func mergeIntoExisting(absorbed, primary, incoming *profileModel.Profile, orgHandle string, score float64,
	schemaRules []schemaModel.ProfileSchemaAttribute, mergeReason, mergeType, mergedBy string) error {

	logger := log.GetLogger()

	other := primary
	if primary.ProfileId == incoming.ProfileId {
		other = absorbed
	}
	mergedProfile := workers.MergeProfiles(*other, *incoming, schemaRules)

	children := []profileModel.Reference{
		{ProfileId: absorbed.ProfileId, Reason: mergeReason},
	}
	if err := profileStore.UpdateProfileReferences(*primary, children); err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: "Failed to update profile references after merge.",
		}, err)
	}

	// Persist merged data section by section — matching the old proven pattern.
	if err := persistMergedDataSections(primary.ProfileId, mergedProfile, logger); err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: fmt.Sprintf("Failed to persist merged data to profile '%s'", primary.ProfileId),
		}, err)
	}

	_ = irStore.InsertMergeAuditLog(model.MergeAuditEntry{
		OrgHandle:          orgHandle,
		PrimaryProfileID:   primary.ProfileId,
		SecondaryProfileID: absorbed.ProfileId,
		MergeType:          mergeType,
		MatchScore:         score,
		MergedBy:           mergedBy,
	})

	ReindexAfterMerge(primary.ProfileId, absorbed.ProfileId, orgHandle, mergedProfile)

	logger.Info(fmt.Sprintf("Worker: merge complete — '%s' merged into '%s'",
		absorbed.ProfileId, primary.ProfileId))
	return nil
}

func MergeWithChildTransfer(incoming, tempMaster *profileModel.Profile, existingRefs []profileModel.Reference,
	orgHandle string, score float64, schemaRules []schemaModel.ProfileSchemaAttribute, mergeReason, mergeType, mergedBy string) error {

	logger := log.GetLogger()

	mergedProfile := workers.MergeProfiles(*tempMaster, *incoming, schemaRules)
	mergedProfile.UserId = incoming.UserId

	// Step 1: Transfer existing children of the temp master to the new perm master.
	if err := profileStore.UpdateProfileReferences(profileModel.Profile{
		ProfileId: incoming.ProfileId,
	}, existingRefs); err != nil {
		logger.Error(fmt.Sprintf("Worker: failed to transfer children from temp master '%s' to perm '%s'",
			tempMaster.ProfileId, incoming.ProfileId), log.Error(err))
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: "Failed to transfer existing children to new permanent master.",
		}, err)
	}

	// Step 2: Add the old temp master itself as a child.
	newChild := []profileModel.Reference{
		{ProfileId: tempMaster.ProfileId, Reason: mergeReason},
	}
	if err := profileStore.UpdateProfileReferences(profileModel.Profile{
		ProfileId: incoming.ProfileId,
	}, newChild); err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: "Failed to add old temp master as child reference.",
		}, err)
	}

	// Step 3: Persist merged data sections.
	if err := persistMergedDataSections(incoming.ProfileId, mergedProfile, logger); err != nil {
		return errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.IR_WORKER_FAILED.Code,
			Message:     errors2.IR_WORKER_FAILED.Message,
			Description: fmt.Sprintf("Failed to persist merged data to profile '%s'", incoming.ProfileId),
		}, err)
	}

	_ = irStore.InsertMergeAuditLog(model.MergeAuditEntry{
		OrgHandle:          orgHandle,
		PrimaryProfileID:   incoming.ProfileId,
		SecondaryProfileID: tempMaster.ProfileId,
		MergeType:          mergeType,
		MatchScore:         score,
		MergedBy:           mergedBy,
	})

	if err := irStore.DeleteBlockingKeys(tempMaster.ProfileId); err != nil {
		logger.Warn(fmt.Sprintf("Worker: failed to delete blocking keys for old temp master '%s'",
			tempMaster.ProfileId), log.Error(err))
	}
	ReindexAfterMerge(incoming.ProfileId, tempMaster.ProfileId, orgHandle, mergedProfile)

	logger.Info(fmt.Sprintf("Worker: perm '%s' took over temp master '%s' — %d children transferred",
		incoming.ProfileId, tempMaster.ProfileId, len(existingRefs)))
	return nil
}

func DetermineMergeOrder(incoming, matched *profileModel.Profile) (*profileModel.Profile, *profileModel.Profile) {
	incomingHasUser := incoming.UserId != ""
	matchedHasUser := matched.UserId != ""

	switch {
	case matchedHasUser && !incomingHasUser:
		return matched, incoming
	case !matchedHasUser && incomingHasUser:
		return incoming, matched
	default:
		return matched, incoming
	}
}

func insertReviewTaskSafe(orgHandle, sourceID, targetID string, score float64, breakdown map[string]float64) {
	logger := log.GetLogger()

	task := model.ReviewTask{
		OrgHandle:       orgHandle,
		SourceProfileID: sourceID,
		TargetProfileID: targetID,
		MatchScore:      score,
		Status:          constants.ReviewStatusPending,
		ScoreBreakdown:  breakdown,
	}

	if err := irStore.InsertReviewTask(task); err != nil {
		logger.Error(fmt.Sprintf("AsyncWorker: failed to create review task for '%s' → '%s'",
			sourceID, targetID), log.Error(err))
	} else {
		logger.Info(fmt.Sprintf("AsyncWorker: review task created for '%s' → '%s' (score=%.4f)",
			sourceID, targetID, score))
	}
}

func ReindexAfterMerge(primaryID, secondaryID, orgHandle string, mergedProfile profileModel.Profile) {
	logger := log.GetLogger()

	if err := irStore.DeleteBlockingKeys(secondaryID); err != nil {
		logger.Warn(fmt.Sprintf("ReindexAfterMerge: failed to delete secondary '%s' blocking keys", secondaryID),
			log.Error(err))
	}

	rawRules, err := urStore.GetUnificationRules(orgHandle)
	if err != nil {
		logger.Warn("ReindexAfterMerge: failed to load rules for re-indexing primary", log.Error(err))
		return
	}

	rules := model.EnrichRulesWithSampling(rawRules, orgHandle, irStore.SampleAttributeValues, buildSchemaLookup)
	if len(rules) == 0 {
		return
	}

	flat := make(map[string]interface{})
	model.FlattenMap("traits", mergedProfile.Traits, flat)
	model.FlattenMap("identity_attributes", mergedProfile.IdentityAttributes, flat)
	if mergedProfile.UserId != "" {
		flat["user_id"] = mergedProfile.UserId
	}

	newKeys := engine.GenerateProfileBlockingKeys(rules, flat)
	if len(newKeys) > 0 {
		if err := irStore.UpsertBlockingKeys(primaryID, orgHandle, newKeys); err != nil {
			logger.Warn(fmt.Sprintf("ReindexAfterMerge: failed to re-index primary '%s'", primaryID),
				log.Error(err))
		} else {
			logger.Info(fmt.Sprintf("ReindexAfterMerge: re-indexed %d blocking keys for primary '%s'",
				len(newKeys), primaryID))
		}
	}
}

func flattenProfile(p *profileModel.Profile) map[string]interface{} {
	flat := make(map[string]interface{})
	model.FlattenMap("traits", p.Traits, flat)
	model.FlattenMap("identity_attributes", p.IdentityAttributes, flat)
	if p.UserId != "" {
		flat["user_id"] = p.UserId
	}
	return flat
}

func persistMergedDataSections(primaryID string, merged profileModel.Profile, logger *log.Logger) error {

	// Section 1: Application Data — loop each app context individually.
	for _, appCtx := range merged.ApplicationData {
		if err := profileStore.InsertMergedMasterProfileAppData(primaryID, appCtx); err != nil {
			logger.Error(fmt.Sprintf("Failed to update app data for master profile: %s", primaryID),
				log.String("appId", appCtx.AppId), log.Error(err))
			return err
		}
	}

	// Section 2: Traits.
	if merged.Traits != nil {
		if err := profileStore.InsertMergedMasterProfileTraitData(primaryID, merged.Traits); err != nil {
			logger.Error(fmt.Sprintf("Failed to update traits for master profile: %s", primaryID),
				log.Error(err))
			return err
		}
	}

	// Section 3: Identity Attributes.
	if merged.IdentityAttributes != nil {
		if err := profileStore.MergeIdentityDataOfProfiles(primaryID, merged.IdentityAttributes); err != nil {
			logger.Error(fmt.Sprintf("Failed to update IdentityData for master profile: %s", primaryID),
				log.Error(err))
			return err
		}
	}

	return nil
}

func PersistMergedData(primaryID string, merged profileModel.Profile) error {
	return persistMergedDataSections(primaryID, merged, log.GetLogger())
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
