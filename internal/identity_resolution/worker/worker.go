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

	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/engine"
	"github.com/wso2/identity-customer-data-service/internal/identity_resolution/model"
	irStore "github.com/wso2/identity-customer-data-service/internal/identity_resolution/store"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	urModel "github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
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

	flatAttrs := flattenProfile(freshProfile)

	// Load unification rules for blocking key generation and scoring/merge decisions.
	rawRules, err := urStore.GetUnificationRules(orgHandle)
	if err != nil {
		logger.Error(fmt.Sprintf("AsyncWorker: failed to load unification rules for org '%s'", orgHandle), log.Error(err))
		return
	}

	rules := filterActiveRules(rawRules)
	if len(rules) == 0 {
		logger.Info(fmt.Sprintf("AsyncWorker: no active unification rules for org '%s', skipping resolution", orgHandle))
		return
	}

	logger.Info(fmt.Sprintf("AsyncWorker: loaded %d active rules", len(rules)))

	// Check if profile has any attributes matching active rules.
	hasMatchingAttr := false
	for _, rule := range rules {
		if v, ok := flatAttrs[rule.PropertyName]; ok && v != nil {
			hasMatchingAttr = true
			break
		}
	}
	if !hasMatchingAttr {
		logger.Info(fmt.Sprintf("AsyncWorker: profile '%s' has no attributes matching active rules, skipping",
			freshProfile.ProfileId))
		return
	}

	// Generate blocking keys from unification rules.
	blockingKeys := engine.GenerateBlockingKeysFromRules(flatAttrs, rules)

	if len(blockingKeys) > 0 {
		if err := irStore.UpsertBlockingKeys(freshProfile.ProfileId, orgHandle, blockingKeys); err != nil {
			logger.Warn(fmt.Sprintf("AsyncWorker: failed to index profile '%s' in blocking_keys",
				freshProfile.ProfileId), log.Error(err))
		} else {
			logger.Info(fmt.Sprintf("AsyncWorker: indexed %d blocking keys for profile '%s'",
				len(blockingKeys), freshProfile.ProfileId))
		}
	}

	excludeID := freshProfile.ProfileId
	parentID := ""
	if freshProfile.ProfileStatus != nil && freshProfile.ProfileStatus.ReferenceProfileId != "" {
		parentID = freshProfile.ProfileStatus.ReferenceProfileId
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
				logger.Info(fmt.Sprintf("AsyncWorker: resolved child candidate '%s' to master '%s'",
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
		mode := model.DetermineMode(inputProfileType, candidateType)

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

				matchedProfile, loadErr := profileStore.GetProfile(sc.id)
				if loadErr != nil || matchedProfile == nil {
					logger.Error(fmt.Sprintf("AsyncWorker: failed to load matched profile '%s' for auto-merge", sc.id))
					insertReviewTaskSafe(orgHandle, freshProfile.ProfileId, sc.id, sc.score, sc.breakdown)
				} else {
					workers.MergeMatchedProfiles(*matchedProfile, *freshProfile, constants.MergeReasonAutoMerge)
					merged = true
					if auditErr := irStore.InsertMergeAuditLog(model.MergeAuditEntry{
						OrgHandle:          orgHandle,
						PrimaryProfileID:   matchedProfile.ProfileId,
						SecondaryProfileID: freshProfile.ProfileId,
						MergeType:          constants.DecisionAutoMerge,
						MatchScore:         sc.score,
						MergedBy:           "SYSTEM",
					}); auditErr != nil {
						logger.Error(fmt.Sprintf("AsyncWorker: failed to insert merge audit log for '%s' → '%s'",
							matchedProfile.ProfileId, freshProfile.ProfileId), log.Error(auditErr))
					}
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
		logger.Warn(fmt.Sprintf("ReindexAfterMerge: failed to load unification rules for org '%s'", orgHandle),
			log.Error(err))
		return
	}
	rules := filterActiveRules(rawRules)

	flat := make(map[string]interface{})
	model.FlattenMap("traits", mergedProfile.Traits, flat)
	model.FlattenMap("identity_attributes", mergedProfile.IdentityAttributes, flat)
	if mergedProfile.UserId != "" {
		flat["user_id"] = mergedProfile.UserId
	}

	newKeys := engine.GenerateBlockingKeysFromRules(flat, rules)
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

// IndexNewAttribute generates blocking keys for a specific attribute across all profiles in an org.
// Called when a unification rule is added or activated.
func IndexNewAttribute(orgHandle string, rule urModel.UnificationRule) {
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Reindexer: indexing attribute '%s' for org '%s'", rule.PropertyName, orgHandle))

	profiles, err := irStore.GetProfilesForOrg(orgHandle)
	if err != nil {
		logger.Error(fmt.Sprintf("Reindexer: failed to load profiles for org '%s'", orgHandle), log.Error(err))
		return
	}

	attrType := rule.AttributeType
	if attrType == "" {
		attrType = constants.AttributeTypePrimitiveExact
	}

	count := 0
	for _, p := range profiles {
		val := ""
		if v, ok := p.Attributes[rule.PropertyName]; ok {
			val = fmt.Sprintf("%v", v)
		}
		if val == "" {
			continue
		}
		keys := engine.GenerateBlockingKeys(attrType, rule.PropertyName, val)
		if len(keys) > 0 {
			if err := irStore.InsertBlockingKeys(p.ProfileID, orgHandle, keys); err != nil {
				logger.Error(fmt.Sprintf("Reindexer: failed to insert keys for profile '%s'", p.ProfileID), log.Error(err))
			}
			count++
		}
	}
	logger.Info(fmt.Sprintf("Reindexer: done indexing attribute '%s', %d profiles indexed", rule.PropertyName, count))
}

// RemoveAttributeIndex removes all blocking keys for a specific attribute in an org.
// Called when a unification rule is deleted or deactivated.
func RemoveAttributeIndex(orgHandle string, attributeName string) {
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Reindexer: removing attribute index '%s' for org '%s'", attributeName, orgHandle))
	if err := irStore.DeleteBlockingKeysByAttribute(orgHandle, attributeName); err != nil {
		logger.Error(fmt.Sprintf("Reindexer: failed to remove attribute index '%s'", attributeName), log.Error(err))
	}
}

// filterActiveRules returns only active unification rules sorted by priority.
func filterActiveRules(rules []urModel.UnificationRule) []urModel.UnificationRule {
	active := make([]urModel.UnificationRule, 0, len(rules))
	for _, r := range rules {
		if r.IsActive {
			if r.AttributeType == "" {
				r.AttributeType = constants.AttributeTypePrimitiveExact
			}
			if r.UnificationMethod == "" {
				r.UnificationMethod = "deterministic"
			}
			active = append(active, r)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].Priority < active[j].Priority
	})
	return active
}
