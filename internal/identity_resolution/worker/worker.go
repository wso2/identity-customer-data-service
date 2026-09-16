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
	"sync"

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
		return
	}

	// Check if profile has any attributes matching active rules.
	hasMatchingAttr := false
	for _, rule := range rules {
		if v, ok := flatAttrs[rule.PropertyName]; ok && v != nil {
			hasMatchingAttr = true
			break
		}
	}
	if !hasMatchingAttr {
		// An update can remove the last value a rule matched on. Returning without touching
		// the index would leave the profile discoverable under its old keys, so it keeps
		// matching on data it no longer has.
		if err := irStore.DeleteBlockingKeys(freshProfile.ProfileId); err != nil {
			logger.Warn(fmt.Sprintf("AsyncWorker: failed to clear stale blocking keys for '%s'",
				freshProfile.ProfileId), log.Error(err))
		}
		return
	}

	// Generate blocking keys from unification rules.
	blockingKeys := engine.GenerateBlockingKeysFromRules(flatAttrs, rules)

	if len(blockingKeys) > 0 {
		if err := irStore.UpsertBlockingKeys(freshProfile.ProfileId, orgHandle, blockingKeys); err != nil {
			logger.Warn(fmt.Sprintf("AsyncWorker: failed to index profile '%s' in blocking_keys",
				freshProfile.ProfileId), log.Error(err))
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
		candidateIDs = filtered
	}

	if len(candidateIDs) == 0 {
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
	// against the same candidate profile because they share the same data.
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
					continue
				}
				// Skip if the master is the parent we already excluded.
				if masterID == parentID {
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
			}
			if !seen[resolvedID] {
				seen[resolvedID] = true
				resolvedIDs = append(resolvedIDs, resolvedID)
			}
		}
		candidateIDs = resolvedIDs
	}

	if len(candidateIDs) == 0 {
		return
	}

	thresholds := model.LoadThresholds(orgHandle)
	scoringCtx := engine.ScoringContext{
		OrgHandle:      orgHandle,
		Thresholds:     thresholds,
		ValueFrequency: irStore.CountProfilesByBlockingKey,
	}

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

		finalScore, breakdown := engine.ScoreCandidate(flatAttrs, candidate, rules, scoringCtx)

		if finalScore >= thresholds.ManualReview {
			scored = append(scored, scoredCandidate{
				id:        candidateID,
				score:     finalScore,
				breakdown: breakdown,
			})
		}
	}

	if len(scored) == 0 {
		return
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Filter out candidates that have been explicitly rejected against this profile.
	rejectedIDs, _ := irStore.GetRejectedProfileIDs(orgHandle, freshProfile.ProfileId)
	if len(rejectedIDs) > 0 {
		var filtered []scoredCandidate
		for _, sc := range scored {
			if _, ok := rejectedIDs[sc.id]; ok {
				continue
			}
			filtered = append(filtered, sc)
		}
		scored = filtered
		if len(scored) == 0 {
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
				continue
			}
			mergeable = append(mergeable, sc)
		}
		scored = mergeable
		if len(scored) == 0 {
			return
		}
	}

	merged := false
	var mergedMaster *profileModel.Profile
	var remaining []scoredCandidate

	for i, sc := range scored {
		decision := model.Decide(sc.score, thresholds)

		if decision == constants.DecisionAutoMerge {

			matchedProfile, loadErr := profileStore.GetProfile(sc.id)
			if loadErr != nil || matchedProfile == nil {
				logger.Error(fmt.Sprintf("AsyncWorker: failed to load matched profile '%s' for auto-merge", sc.id))
				insertReviewTask(orgHandle, freshProfile.ProfileId, sc.id, sc.score, sc.breakdown)
				continue
			}

			// Attribute the merge to the rule that actually drove it, so the child
			// reference records why rather than a generic "auto_merge".
			mergeReason := constants.MergeReasonAutoMerge
			for _, rule := range rules {
				if score, ok := sc.breakdown[rule.PropertyName]; ok && score == sc.score {
					mergeReason = rule.RuleName
					break
				}
			}

			// The surviving master is whichever profile the merge promoted — it is not
			// necessarily matchedProfile. A permanent profile wins over a temporary one,
			// and two temporary profiles are both demoted under a brand-new master.
			survivingMaster, mergeErr := workers.MergeMatchedProfiles(*matchedProfile, *freshProfile, mergeReason)
			if mergeErr != nil {
				// Merge failed so not mark merged=true, not write audit log, not
				// cascade-cancel related tasks.
				logger.Error(fmt.Sprintf("AsyncWorker: auto-merge failed for '%s' → '%s' — falling back to review task",
					matchedProfile.ProfileId, freshProfile.ProfileId), log.Error(mergeErr))
				insertReviewTask(orgHandle, freshProfile.ProfileId, sc.id, sc.score, sc.breakdown)
				continue
			}
			if survivingMaster == nil {
				// Pair turned out to be unmergeable — surface it for review instead.
				insertReviewTask(orgHandle, freshProfile.ProfileId, sc.id, sc.score, sc.breakdown)
				continue
			}
			merged = true
			mergedMaster = survivingMaster
			if auditErr := irStore.InsertMergeAuditLog(model.MergeAuditEntry{
				OrgHandle:          orgHandle,
				PrimaryProfileID:   survivingMaster.ProfileId,
				SecondaryProfileID: freshProfile.ProfileId,
				MergeType:          constants.DecisionAutoMerge,
				MatchScore:         sc.score,
				MergedBy:           constants.MergeOnTrigger,
			}); auditErr != nil {
				logger.Error(fmt.Sprintf("AsyncWorker: failed to insert merge audit log for '%s' → '%s'",
					survivingMaster.ProfileId, freshProfile.ProfileId), log.Error(auditErr))
			}
			remaining = scored[i+1:]
			break
		}

		insertReviewTask(orgHandle, freshProfile.ProfileId, sc.id, sc.score, sc.breakdown)
	}

	if merged {
		// Cancel pending review tasks that referenced the freshly-merged profile.
		cancelledIncomingIDs, cancelErr := irStore.CancelRelatedReviewTasks("", freshProfile.ProfileId,
			mergedMaster.ProfileId, constants.CanceledBySystem)
		if cancelErr != nil {
			logger.Warn(fmt.Sprintf("AsyncWorker: cascade cancel failed after auto-merge of '%s' → '%s'",
				freshProfile.ProfileId, mergedMaster.ProfileId), log.Error(cancelErr))
		}
		for _, srcID := range cancelledIncomingIDs {
			p, loadErr := profileStore.GetProfile(srcID)
			if loadErr != nil || p == nil {
				continue
			}
			// If the cancelled task's incoming has itself become a child, re-enqueue its master instead.
			if p.ProfileStatus != nil && p.ProfileStatus.ReferenceProfileId != "" {
				masterID := p.ProfileStatus.ReferenceProfileId
				if masterID == mergedMaster.ProfileId {
					continue
				}
				master, mErr := profileStore.GetProfile(masterID)
				if mErr != nil || master == nil {
					continue
				}
				workers.EnqueueProfileForProcessing(*master)
				continue
			}
			workers.EnqueueProfileForProcessing(*p)
		}

		// Surface remaining qualified candidates as review tasks against the new master.
		// Incoming = mergedMaster.ProfileId, the new master is the live entity that holds the matched attributes.
		for _, sc := range remaining {
			decision := model.Decide(sc.score, thresholds)
			if decision == constants.DecisionAutoMerge || decision == constants.DecisionManualReview {
				insertReviewTask(orgHandle, mergedMaster.ProfileId, sc.id, sc.score, sc.breakdown)
			}
		}
	}
}

func insertReviewTask(orgHandle, incomingProfileID, candidateProfileID string, score float64, breakdown map[string]float64) {
	logger := log.GetLogger()

	rejectedIDs, err := irStore.GetRejectedProfileIDs(orgHandle, incomingProfileID)
	if err != nil {
		logger.Warn(fmt.Sprintf("AsyncWorker: could not check rejection pairs for '%s', proceeding with task creation", incomingProfileID), log.Error(err))
	} else if _, ok := rejectedIDs[candidateProfileID]; ok {
		return
	}

	task := model.ReviewTask{
		OrgHandle:          orgHandle,
		IncomingProfileID:  incomingProfileID,
		CandidateProfileID: candidateProfileID,
		MatchScore:         score,
		Status:             constants.ReviewStatusPending,
		ScoreBreakdown:     breakdown,
	}

	if err := irStore.InsertReviewTask(task); err != nil {
		logger.Error(fmt.Sprintf("AsyncWorker: failed to create review task for '%s' → '%s'",
			incomingProfileID, candidateProfileID), log.Error(err))
	}
}

func ReindexAfterMerge(masterProfileID, triggerProfileId, orgHandle string, mergedProfile profileModel.Profile) {
	logger := log.GetLogger()

	if err := irStore.DeleteBlockingKeys(triggerProfileId); err != nil {
		logger.Warn(fmt.Sprintf("ReindexAfterMerge: failed to delete trigger '%s' blocking keys", triggerProfileId),
			log.Error(err))
	}

	// The merged-away profile is no longer the addressable entity, so any rejection naming
	// it has to follow it onto the master or the dismissed pair comes straight back.
	if err := irStore.RepointRejectionPairs(orgHandle, triggerProfileId, masterProfileID); err != nil {
		logger.Warn(fmt.Sprintf("ReindexAfterMerge: failed to repoint rejection pairs from '%s' to '%s'",
			triggerProfileId, masterProfileID), log.Error(err))
	}

	rawRules, err := urStore.GetUnificationRules(orgHandle)
	if err != nil {
		logger.Warn(fmt.Sprintf("ReindexAfterMerge: failed to load unification rules for org '%s'", orgHandle),
			log.Error(err))
		return
	}
	rules := filterActiveRules(rawRules)

	newKeys := engine.GenerateBlockingKeysFromRules(flattenProfile(&mergedProfile), rules)
	if len(newKeys) > 0 {
		if err := irStore.UpsertBlockingKeys(masterProfileID, orgHandle, newKeys); err != nil {
			logger.Warn(fmt.Sprintf("ReindexAfterMerge: failed to re-index master '%s'", masterProfileID),
				log.Error(err))
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

// backfillsInFlight prevents two backfills of the same attribute running at once. Rapid
// toggling of a rule would otherwise start overlapping full-org scans that duplicate each
// other's work and compete for connections.
var backfillsInFlight sync.Map

// IndexNewAttribute generates blocking keys for a specific attribute across all profiles in
// an org. Called when a unification rule is added or activated.
//
// The scan is keyed rather than offset-based so concurrent profile inserts cannot shift a
// row out of the window unread, and progress is logged at both ends because nothing else
// reports on it: a backfill that dies halfway leaves a partially indexed attribute that
// looks exactly like a fully indexed one, and silently under-matches until the next time
// every affected profile happens to be updated.
func IndexNewAttribute(orgHandle string, rule urModel.UnificationRule) {
	logger := log.GetLogger()

	inFlightKey := orgHandle + "|" + rule.PropertyName
	if _, running := backfillsInFlight.LoadOrStore(inFlightKey, true); running {
		logger.Info(fmt.Sprintf("Reindexer: backfill already running for '%s' in org '%s', skipping",
			rule.PropertyName, orgHandle))
		return
	}
	defer backfillsInFlight.Delete(inFlightKey)

	attrType := rule.AttributeType
	if attrType == "" {
		attrType = constants.AttributeTypePrimitiveExact
	}
	method := rule.UnificationMethod
	if method == "" {
		method = constants.UnificationMethodDeterministic
	}

	logger.Info(fmt.Sprintf("Reindexer: starting backfill of '%s' for org '%s'",
		rule.PropertyName, orgHandle))

	totalIndexed, totalScanned := 0, 0
	afterProfileID := ""

	for {
		profiles, err := irStore.GetProfilesForOrgAfter(orgHandle, afterProfileID, constants.GetProfilesPageSize)
		if err != nil {
			logger.Error(fmt.Sprintf(
				"Reindexer: backfill of '%s' for org '%s' ABORTED after %d profiles — the index is incomplete",
				rule.PropertyName, orgHandle, totalScanned), log.Error(err))
			return
		}
		if len(profiles) == 0 {
			break
		}

		perProfileKeys := make(map[string][]model.BlockingKey, len(profiles))
		for _, p := range profiles {
			afterProfileID = p.ProfileID
			totalScanned++

			values := p.GetAllAttributeValues(rule.PropertyName)
			if len(values) == 0 {
				continue
			}
			var keys []model.BlockingKey
			for _, val := range values {
				keys = append(keys, engine.GenerateBlockingKeys(attrType, method, rule.PropertyName, val)...)
			}
			if len(keys) > 0 {
				perProfileKeys[p.ProfileID] = keys
			}
		}

		if len(perProfileKeys) > 0 {
			if err := irStore.InsertBlockingKeysBatch(orgHandle, perProfileKeys); err != nil {
				logger.Error(fmt.Sprintf(
					"Reindexer: batch insert failed for org '%s' at profile '%s' — those profiles are unindexed",
					orgHandle, afterProfileID), log.Error(err))
			} else {
				totalIndexed += len(perProfileKeys)
			}
		}

		if len(profiles) < constants.GetProfilesPageSize {
			break
		}
	}

	logger.Info(fmt.Sprintf("Reindexer: backfill of '%s' for org '%s' complete — %d of %d profiles indexed",
		rule.PropertyName, orgHandle, totalIndexed, totalScanned))
}

// RemoveAttributeIndex removes all blocking keys for a specific attribute in an org.
// Called when a unification rule is deleted or deactivated.
func RemoveAttributeIndex(orgHandle string, attributeName string) {
	logger := log.GetLogger()
	if err := irStore.DeleteBlockingKeysByAttribute(orgHandle, attributeName); err != nil {
		logger.Error(fmt.Sprintf("Reindexer: failed to remove attribute index '%s'", attributeName), log.Error(err))
	}
}

// filterActiveRules returns only active unification rules sorted by priority.
func filterActiveRules(rules []urModel.UnificationRule) []urModel.UnificationRule {
	active := make([]urModel.UnificationRule, 0, len(rules))
	for _, r := range rules {
		if !r.IsActive {
			continue
		}
		active = append(active, urModel.ApplyDefaults(r, constants.AttributeTypePrimitiveExact,
			constants.UnificationMethodDeterministic, constants.DefaultMatchStrength,
			constants.DefaultMismatchStrength))
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].Priority < active[j].Priority
	})
	return active
}
