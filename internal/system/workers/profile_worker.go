/*
 * Copyright (c) 2025-2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package workers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaStore "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"github.com/wso2/identity-customer-data-service/internal/system/config"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/queue"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/provider"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// activeProfileQueue is the queue implementation used for profile unification.
// It is initialised by StartProfileWorker. All access is guarded by
// profileQueueMu to prevent data races between concurrent Enqueue calls and
// shutdown.
var (
	profileQueueMu     sync.RWMutex
	activeProfileQueue queue.ProfileUnificationQueue
)

// StartProfileWorker initialises the profile unification queue (using the
// provider configured in the runtime config) and starts the consumer
// goroutine. An error is returned when the queue cannot be created or
// started; the caller should treat this as a fatal startup failure.
func StartProfileWorker() error {
	cfg := config.GetCDSRuntime().Config.MessageQueue
	q, err := queue.NewProfileUnificationQueue(cfg)
	if err != nil {
		return fmt.Errorf("workers: failed to create profile unification queue: %w", err)
	}
	if err := q.Start(func(profile profileModel.Profile) {
		p, err := profileStore.GetProfile(profile.ProfileId)
		if err == nil && p != nil {
			unifyProfiles(*p)
		}
	}); err != nil {
		_ = q.Close()
		return fmt.Errorf("workers: failed to start profile unification queue: %w", err)
	}
	profileQueueMu.Lock()
	activeProfileQueue = q
	profileQueueMu.Unlock()
	return nil
}

// EnqueueProfileForProcessing adds a profile to the active queue for
// asynchronous unification. It is a no-op when the worker has not been
// started or has been stopped.
func EnqueueProfileForProcessing(profile profileModel.Profile) {
	profileQueueMu.RLock()
	q := activeProfileQueue
	profileQueueMu.RUnlock()
	if q != nil {
		if err := q.Enqueue(profile); err != nil {
			log.GetLogger().Error(fmt.Sprintf(
				"workers: failed to enqueue profile %s for unification: %v",
				profile.ProfileId, err))
		}
	}
}

// ProfileWorkerQueue is a thin adapter that allows service-layer code to
// enqueue profiles without taking a direct dependency on the queue package.
type ProfileWorkerQueue struct{}

// Enqueue forwards the profile to the active queue via EnqueueProfileForProcessing.
func (q *ProfileWorkerQueue) Enqueue(profile profileModel.Profile) {
	EnqueueProfileForProcessing(profile)
}

// StopProfileWorker gracefully shuts down the profile unification queue.
// It nils out the global reference under a write lock before calling Close,
// ensuring no concurrent Enqueue can send on a closed queue. It should be
// called during application shutdown.
func StopProfileWorker() error {
	profileQueueMu.Lock()
	q := activeProfileQueue
	activeProfileQueue = nil
	profileQueueMu.Unlock()
	if q != nil {
		return q.Close()
	}
	return nil
}

// unifyProfiles unifies profiles based on unification rules
func unifyProfiles(newProfile profileModel.Profile) {

	logger := log.GetLogger()

	// Step 1: Fetch all unification rules
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	unificationRules, err := ruleService.GetUnificationRules(newProfile.OrgHandle)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch unification rules for unifying profile: %s",
			newProfile.ProfileId), log.Error(err))
	}
	if len(unificationRules) == 0 {
		logger.Info(fmt.Sprintf("No unification rules found for tenant: %s", newProfile.OrgHandle))
	}

	logger.Info(fmt.Sprintf("Beginning to evaluate unification for profile: %s", newProfile.ProfileId))

	// Step 2: Fetch all existing profiles from DB
	existingMasterProfiles, err := profileStore.GetAllReferenceProfilesExceptForCurrent(newProfile)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch existing master profiles for unification of profile: %s",
			newProfile.ProfileId), log.Error(err))
		return
	}

	// Step 3a: Direct userId match — system-level invariant, no rule needed.
	// Profiles with the same userId must always be merged.
	if newProfile.UserId != "" {
		for _, existingMasterProfile := range existingMasterProfiles {
			if existingMasterProfile.ProfileId == newProfile.ProfileStatus.ReferenceProfileId {
				continue
			}
			if existingMasterProfile.UserId == newProfile.UserId {
				logger.Info(fmt.Sprintf("Profiles %s and %s share the same userId %s. Proceeding with merge.",
					existingMasterProfile.ProfileId, newProfile.ProfileId, newProfile.UserId))
				mergeMatchedProfiles(existingMasterProfile, newProfile, constants.SystemUserIdMatchReason)
				return
			}
		}
	}

	// Step 3b: Rule-based matching (email, phone, etc.)
	unificationRules = filterActiveRulesAndSortByPriority(unificationRules)
	for _, rule := range unificationRules {
		for _, existingMasterProfile := range existingMasterProfiles {
			if existingMasterProfile.ProfileId == newProfile.ProfileStatus.ReferenceProfileId {
				// Skip if the existing master profile is the parent of the new profile
				return
			}
			if doesProfileMatch(existingMasterProfile, newProfile, rule) {
				mergeMatchedProfiles(existingMasterProfile, newProfile, rule.RuleName)
				return
			}
		}
	}
}

// mergeMatchedProfiles handles all merge scenarios for two matched profiles.
// It determines the master/child relationship based on permanent (has userId) vs temporary,
// and whether the existing profile already has child references.
func mergeMatchedProfiles(existingMasterProfile profileModel.Profile, newProfile profileModel.Profile, reason string) {

	logger := log.GetLogger()

	// Fetch references for the existing master profile
	refs, err := profileStore.FetchReferencedProfiles(existingMasterProfile.ProfileId)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch references for profile %s during unification with profile %s", existingMasterProfile.ProfileId))
	}
	existingMasterProfile.ProfileStatus.References = refs

	// Merge profile data using schema rules
	schemaRules, err := schemaStore.GetProfileSchemaAttributesForOrg(newProfile.OrgHandle)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch profile schema attributes for org %s during unification of profile %s", newProfile.OrgHandle, newProfile.ProfileId))
	}
	newMasterProfile := MergeProfiles(existingMasterProfile, newProfile, schemaRules)

	hasUserIDExisting := existingMasterProfile.UserId != ""
	hasUserIDNew := newProfile.UserId != ""
	hasExistingChildren := len(existingMasterProfile.ProfileStatus.References) > 0 &&
		existingMasterProfile.ProfileStatus.IsReferenceProfile

	// ── Case: Both permanent with different userIds — do not merge ──
	if hasUserIDExisting && hasUserIDNew && existingMasterProfile.UserId != newProfile.UserId {
		logger.Info(fmt.Sprintf("Not merging profiles %s and %s — different userIds (%s vs %s)",
			existingMasterProfile.ProfileId, newProfile.ProfileId,
			existingMasterProfile.UserId, newProfile.UserId))
		return
	}

	// ── Case: perm-temp or temp-perm ──
	if hasUserIDExisting != hasUserIDNew {
		mergePermanentAndTemporary(existingMasterProfile, newProfile, newMasterProfile, reason, hasExistingChildren)
		return
	}

	// ── Case: Both permanent with same userId OR both temporary ──
	mergeSameKindProfiles(existingMasterProfile, newProfile, newMasterProfile, reason, hasUserIDExisting, hasExistingChildren)
}

// mergePermanentAndTemporary merges a permanent profile (has userId) with a temporary one.
// The permanent profile always becomes the master.
func mergePermanentAndTemporary(
	existingMasterProfile profileModel.Profile,
	newProfile profileModel.Profile,
	newMasterProfile profileModel.Profile,
	reason string,
	hasExistingChildren bool,
) {
	logger := log.GetLogger()

	hasUserIDExisting := existingMasterProfile.UserId != ""

	if hasUserIDExisting {
		// Existing is permanent — it stays as master, new becomes child
		logger.Info(fmt.Sprintf("Stitching temporary profile %s to permanent profile %s",
			newProfile.ProfileId, existingMasterProfile.ProfileId))

		newMasterProfile.ProfileId = existingMasterProfile.ProfileId
		newMasterProfile.UserId = existingMasterProfile.UserId

		newChild := profileModel.Reference{
			ProfileId: newProfile.ProfileId,
			Reason:    reason,
		}
		children := []profileModel.Reference{newChild}

		if err := profileStore.UpdateProfileReferences(newMasterProfile, children); err != nil {
			logger.Error(fmt.Sprintf("Failed to add child profile %s to master %s",
				newProfile.ProfileId, newMasterProfile.ProfileId), log.Error(err))
			return
		}
	} else {
		// New is permanent — it becomes master, existing becomes child
		logger.Info(fmt.Sprintf("Stitching temporary profile %s to permanent profile %s",
			existingMasterProfile.ProfileId, newProfile.ProfileId))

		newMasterProfile.ProfileId = newProfile.ProfileId
		newMasterProfile.UserId = newProfile.UserId

		// If the existing master had children, re-parent them to the new master
		if hasExistingChildren {
			if err := profileStore.UpdateProfileReferences(newMasterProfile, existingMasterProfile.ProfileStatus.References); err != nil {
				logger.Error(fmt.Sprintf("Failed to re-parent references from %s to %s",
					existingMasterProfile.ProfileId, newMasterProfile.ProfileId), log.Error(err))
				return
			}
		}

		newChild := profileModel.Reference{
			ProfileId: existingMasterProfile.ProfileId,
			Reason:    reason,
		}
		children := []profileModel.Reference{newChild}

		if err := profileStore.UpdateProfileReferences(newMasterProfile, children); err != nil {
			logger.Error(fmt.Sprintf("Failed to add child profile %s to master %s",
				existingMasterProfile.ProfileId, newMasterProfile.ProfileId), log.Error(err))
			return
		}
	}

	// Write merged data to the master profile
	persistMergedProfileData(newMasterProfile, newProfile.ProfileId)
}

// mergeSameKindProfiles merges two profiles of the same kind:
// both permanent (same userId) or both temporary.
func mergeSameKindProfiles(
	existingMasterProfile profileModel.Profile,
	newProfile profileModel.Profile,
	newMasterProfile profileModel.Profile,
	reason string,
	bothPermanent bool,
	hasExistingChildren bool,
) {
	logger := log.GetLogger()

	if hasExistingChildren {
		// Existing master already has children — merge new profile into it as another child.
		if bothPermanent {
			logger.Info(fmt.Sprintf("Both profiles are permanent with same userId. Merging %s into existing master %s",
				newProfile.ProfileId, existingMasterProfile.ProfileId))
		} else {
			logger.Info(fmt.Sprintf("Both profiles are temporary. Merging %s into existing master %s",
				newProfile.ProfileId, existingMasterProfile.ProfileId))
		}

		newMasterProfile.ProfileId = existingMasterProfile.ProfileId
		newMasterProfile.UserId = existingMasterProfile.UserId

		newChild := profileModel.Reference{
			ProfileId: newProfile.ProfileId,
			Reason:    reason,
		}
		children := []profileModel.Reference{newChild}

		if err := profileStore.UpdateProfileReferences(newMasterProfile, children); err != nil {
			logger.Error(fmt.Sprintf("Failed to add child profile %s to master %s",
				newProfile.ProfileId, newMasterProfile.ProfileId), log.Error(err))
			return
		}
	} else if bothPermanent {
		// Both permanent, same userId, no children — promote existing as master.
		logger.Info(fmt.Sprintf("Both profiles are permanent with same userId. Merging %s into existing master %s",
			newProfile.ProfileId, existingMasterProfile.ProfileId))

		newMasterProfile.ProfileId = existingMasterProfile.ProfileId
		newMasterProfile.UserId = existingMasterProfile.UserId

		newChild := profileModel.Reference{
			ProfileId: newProfile.ProfileId,
			Reason:    reason,
		}
		children := []profileModel.Reference{newChild}

		if err := profileStore.UpdateProfileReferences(newMasterProfile, children); err != nil {
			logger.Error(fmt.Sprintf("Failed to add child profile %s to master %s",
				newProfile.ProfileId, newMasterProfile.ProfileId), log.Error(err))
			return
		}
	} else {
		// Both temporary, no children — create a new neutral master referencing both.
		logger.Info(fmt.Sprintf("Both profiles are temporary. Creating new master for %s and %s",
			newProfile.ProfileId, existingMasterProfile.ProfileId))

		newMasterProfile.ProfileId = uuid.New().String()
		newMasterProfile.UserId = ""
		newMasterProfile.Location = utils.BuildProfileLocation(newMasterProfile.OrgHandle, newMasterProfile.ProfileId)

		childProfile1 := profileModel.Reference{
			ProfileId: newProfile.ProfileId,
			Reason:    reason,
		}
		childProfile2 := profileModel.Reference{
			ProfileId: existingMasterProfile.ProfileId,
			Reason:    reason,
		}

		newMasterProfile.ProfileStatus = &profileModel.ProfileStatus{
			IsReferenceProfile: true,
			ListProfile:        false,
			References:         []profileModel.Reference{childProfile1, childProfile2},
		}

		if err := profileStore.InsertProfile(newMasterProfile); err != nil {
			_ = profileStore.DeleteProfile(newMasterProfile.ProfileId) // cleanup
			logger.Error(fmt.Sprintf("Failed to insert new master profile while unifying %s and %s",
				newProfile.ProfileId, existingMasterProfile.ProfileId), log.Error(err))
			return
		}

		children := []profileModel.Reference{childProfile1, childProfile2}
		if err := profileStore.UpdateProfileReferences(newMasterProfile, children); err != nil {
			logger.Error(fmt.Sprintf("Failed to add child profiles to new master %s",
				newMasterProfile.ProfileId), log.Error(err))
			return
		}
	}

	// Write merged data to the master profile
	persistMergedProfileData(newMasterProfile, newProfile.ProfileId)
}

// persistMergedProfileData writes the merged application data, traits, and identity attributes
// to the master profile in the store.
func persistMergedProfileData(masterProfile profileModel.Profile, triggerProfileId string) {

	logger := log.GetLogger()

	// Update ApplicationData
	for _, appCtx := range masterProfile.ApplicationData {
		if err := profileStore.InsertMergedMasterProfileAppData(masterProfile.ProfileId, appCtx); err != nil {
			logger.Error(fmt.Sprintf("Failed to update app data for master profile %s while unifying profile %s",
				masterProfile.ProfileId, triggerProfileId), log.Error(err))
			return
		}
	}

	// Update Traits
	if masterProfile.Traits != nil {
		if err := profileStore.InsertMergedMasterProfileTraitData(masterProfile.ProfileId, masterProfile.Traits); err != nil {
			logger.Error(fmt.Sprintf("Failed to update traits for master profile %s while unifying profile %s",
				masterProfile.ProfileId, triggerProfileId), log.Error(err))
			return
		}
	}

	// Update Identity Attributes
	if masterProfile.IdentityAttributes != nil {
		if err := profileStore.MergeIdentityDataOfProfiles(masterProfile.ProfileId, masterProfile.IdentityAttributes); err != nil {
			logger.Error(fmt.Sprintf("Failed to update identity data for master profile %s while unifying profile %s",
				masterProfile.ProfileId, triggerProfileId), log.Error(err))
			return
		}
	}
}

func filterActiveRulesAndSortByPriority(rules []model.UnificationRule) []model.UnificationRule {
	activeRules := make([]model.UnificationRule, 0, len(rules))
	for _, r := range rules {
		if r.IsActive {
			activeRules = append(activeRules, r)
		}
	}
	sort.Slice(activeRules, func(i, j int) bool {
		return activeRules[i].Priority < activeRules[j].Priority
	})
	return activeRules
}

// MergeProfiles merges two profiles based on schema rules
func MergeProfiles(existingProfile profileModel.Profile, incomingProfile profileModel.Profile, schemaRules []schemaModel.ProfileSchemaAttribute) profileModel.Profile {

	logger := log.GetLogger()
	logger.Info("Merging profiles, " + existingProfile.ProfileId + " and " + incomingProfile.ProfileId)
	merged := existingProfile

	for _, rule := range schemaRules {
		traitPath := strings.Split(rule.AttributeName, ".")
		if len(traitPath) < 2 {
			continue
		}
		traitNamespace := traitPath[0]
		propertyName := traitPath[1]

		var existingVal, newVal interface{}
		switch traitNamespace {
		case "traits":
			if existingProfile.Traits != nil {
				existingVal = existingProfile.Traits[propertyName]
			}
			if incomingProfile.Traits != nil {
				newVal = incomingProfile.Traits[propertyName]
			}
		case "identity_attributes":
			if existingProfile.IdentityAttributes != nil {
				existingVal = existingProfile.IdentityAttributes[propertyName]
			}
			if incomingProfile.IdentityAttributes != nil {
				newVal = incomingProfile.IdentityAttributes[propertyName]
			}
		}

		mergedVal := MergeTraitValue(existingVal, newVal, rule.MergeStrategy, rule.ValueType, rule.MultiValued)

		if mergedVal == nil || mergedVal == "" {
			continue
		}

		switch traitNamespace {
		case "traits":
			if merged.Traits == nil {
				merged.Traits = map[string]interface{}{}
			}
			merged.Traits[propertyName] = mergedVal
		case "identity_attributes":
			if merged.IdentityAttributes == nil {
				merged.IdentityAttributes = map[string]interface{}{}
			}
			merged.IdentityAttributes[propertyName] = mergedVal
		}
	}

	merged.ApplicationData = mergeAppData(existingProfile.ApplicationData, incomingProfile.ApplicationData, schemaRules)

	if incomingProfile.UserId != "" {
		merged.UserId = incomingProfile.UserId
	}
	if existingProfile.UserId != "" {
		merged.UserId = existingProfile.UserId
	}

	merged.OrgHandle = incomingProfile.OrgHandle
	merged.CreatedAt = existingProfile.CreatedAt
	merged.UpdatedAt = time.Now().UTC()

	return merged
}

// doesProfileMatch checks if two profiles have matching attributes based on a unification rule
func doesProfileMatch(existingProfile profileModel.Profile, newProfile profileModel.Profile, rule model.UnificationRule) bool {

	log.GetLogger().Debug(fmt.Sprintf("Checking if profiles match for existing id: %s, new id: %s for the rule: %s",
		existingProfile.ProfileId, newProfile.ProfileId, rule.RuleName))
	existingJSON, _ := json.Marshal(existingProfile)
	newJSON, _ := json.Marshal(newProfile)
	existingValues := extractFieldFromJSON(existingJSON, rule.PropertyName)
	newValues := extractFieldFromJSON(newJSON, rule.PropertyName)
	logger := log.GetLogger()
	if checkForMatch(existingValues, newValues) {
		logger.Info(fmt.Sprintf("Profiles %s, %s has matched for unification rule: %s ", existingProfile.ProfileId,
			newProfile.ProfileId, rule.RuleName))
		return true
	}
	return false
}

// extractFieldFromJSON extracts a nested field from raw JSON (`[]byte`) without pre-converting to a map
func extractFieldFromJSON(jsonData []byte, fieldPath string) []interface{} {
	var jsonObj interface{}
	err := json.Unmarshal(jsonData, &jsonObj)
	if err != nil {
		return nil
	}
	return getNestedJSONField(jsonObj, fieldPath)
}

// getNestedJSONField retrieves a nested field from a parsed JSON object
func getNestedJSONField(jsonObj interface{}, fieldPath string) []interface{} {
	fields := strings.Split(fieldPath, ".")
	var value interface{} = jsonObj

	for _, field := range fields {
		if nestedMap, ok := value.(map[string]interface{}); ok {
			value = nestedMap[field]
		} else if nestedSlice, ok := value.([]interface{}); ok {
			var results []interface{}
			for _, item := range nestedSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					extracted := getNestedJSONField(itemMap, strings.Join(fields[1:], "."))
					results = append(results, extracted...)
				}
			}
			return results
		} else {
			return nil
		}
	}

	if list, ok := value.([]interface{}); ok {
		return list
	}

	return []interface{}{value}
}

// checkForMatch checks if at least one value from `newProfile` exists in `existingProfile`
func checkForMatch(existingValues, newValues []interface{}) bool {
	existingSet := make(map[string]bool)
	for _, val := range existingValues {
		if str, ok := val.(string); ok {
			existingSet[str] = true
		}
	}

	for _, val := range newValues {
		if str, ok := val.(string); ok {
			if existingSet[str] {
				return true
			}
		}
	}
	return false
}

func MergeTraitValue(existing interface{}, incoming interface{}, strategy string, valueType string, multiValued bool) interface{} {

	switch strings.ToLower(strategy) {
	case "overwrite":
		if incoming == nil {
			return existing
		}
		if incoming == "" {
			return existing
		}
		return incoming

	case "ignore":
		if existing != nil {
			return existing
		}
		return incoming

	case "combine":
		if !multiValued {
			log.GetLogger().Warn("Merge strategy 'combine' is used for a non-multi-valued field. ")
			return incoming
		}
		switch strings.ToLower(valueType) {
		case "text", "string":
			a := toStringSlice(existing)
			b := toStringSlice(incoming)
			return combineUniqueStrings(a, b)

		case "integer":
			a := toIntSlice(existing)
			b := toIntSlice(incoming)
			return combineUniqueInts(a, b)

		case "decimal":
			a := toFloatSlice(existing)
			b := toFloatSlice(incoming)
			return combineUniqueFloats(a, b)

		case "boolean":
			a := toBoolSlice(existing)
			b := toBoolSlice(incoming)
			return combineUniqueBools(a, b)

		case "date_time", "datetime":
			a := toStringSlice(existing)
			b := toStringSlice(incoming)
			return combineUniqueStrings(a, b)

		case "object":
			a, okA := existing.([]interface{})
			b, okB := incoming.([]interface{})
			if okA && okB {
				return append(a, b...)
			}
			return incoming

		default:
			return incoming
		}
	default:
		return incoming
	}
}

func toStringSlice(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return v
	case string:
		return []string{v}
	case []interface{}:
		var result []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case primitive.A:
		var result []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return []string{}
	}
}

func toIntSlice(value interface{}) []int {
	switch v := value.(type) {
	case []int:
		return v
	case []interface{}:
		result := make([]int, 0, len(v))
		for _, item := range v {
			if i, ok := item.(float64); ok {
				result = append(result, int(i))
			} else if i, ok := item.(int); ok {
				result = append(result, i)
			}
		}
		return result
	case int:
		return []int{v}
	case float64:
		return []int{int(v)}
	default:
		return []int{}
	}
}

func combineUniqueStrings(a, b []string) []string {
	seen := make(map[string]bool)
	var combined []string
	for _, val := range append(a, b...) {
		if !seen[val] {
			seen[val] = true
			combined = append(combined, val)
		}
	}
	return combined
}

func combineUniqueInts(a, b []int) []int {
	seen := make(map[int]bool)
	var combined []int
	for _, val := range append(a, b...) {
		if !seen[val] {
			seen[val] = true
			combined = append(combined, val)
		}
	}
	return combined
}

func toFloatSlice(value interface{}) []float64 {
	switch v := value.(type) {
	case []float64:
		return v
	case []interface{}:
		var result []float64
		for _, item := range v {
			switch i := item.(type) {
			case float64:
				result = append(result, i)
			case int:
				result = append(result, float64(i))
			}
		}
		return result
	case float64:
		return []float64{v}
	case int:
		return []float64{float64(v)}
	default:
		return []float64{}
	}
}

func toBoolSlice(value interface{}) []bool {
	switch v := value.(type) {
	case []bool:
		return v
	case []interface{}:
		var result []bool
		for _, item := range v {
			if b, ok := item.(bool); ok {
				result = append(result, b)
			}
		}
		return result
	case bool:
		return []bool{v}
	default:
		return []bool{}
	}
}

func combineUniqueFloats(a, b []float64) []float64 {
	seen := make(map[float64]bool)
	var combined []float64
	for _, val := range append(a, b...) {
		if !seen[val] {
			seen[val] = true
			combined = append(combined, val)
		}
	}
	return combined
}

func combineUniqueBools(a, b []bool) []bool {
	seen := make(map[bool]bool)
	var combined []bool
	for _, val := range append(a, b...) {
		if !seen[val] {
			seen[val] = true
			combined = append(combined, val)
		}
	}
	return combined
}

func mergeAppData(existingAppData, incomingAppData []profileModel.ApplicationData, rules []schemaModel.ProfileSchemaAttribute) []profileModel.ApplicationData {

	logger := log.GetLogger()
	mergedMap := make(map[string]profileModel.ApplicationData)

	for _, app := range existingAppData {
		mergedMap[app.AppId] = app
		logger.Info("Merging existing application data for application: " + app.AppId)
	}

	for _, newApp := range incomingAppData {
		existingApp, found := mergedMap[newApp.AppId]
		logger.Info("Merging existing application data for application: " + newApp.AppId)
		if !found {
			mergedMap[newApp.AppId] = newApp
			continue
		}

		if existingApp.AppSpecificData == nil {
			existingApp.AppSpecificData = map[string]interface{}{}
		}
		if newApp.AppSpecificData != nil {
			for key, newVal := range newApp.AppSpecificData {
				existingVal := existingApp.AppSpecificData[key]

				strategy := ""
				valueType := ""
				multiValued := false

				for _, r := range rules {
					if r.AttributeName == fmt.Sprintf("application_data.%s", key) {
						strategy = r.MergeStrategy
						valueType = r.ValueType
						multiValued = r.MultiValued
						break
					}
				}

				mergedVal := MergeTraitValue(existingVal, newVal, strategy, valueType, multiValued)
				existingApp.AppSpecificData[key] = mergedVal
			}
		} else {
			logger.Warn(fmt.Sprintf("No app-specific data for application: %s", newApp.AppId))
		}

		mergedMap[newApp.AppId] = existingApp
	}

	var mergedList []profileModel.ApplicationData
	for appID, app := range mergedMap {
		logger.Info(fmt.Sprintf("Merged application data for application: %s", appID))
		mergedList = append(mergedList, app)
	}

	logger.Info(fmt.Sprintf("Application data merge completed for %d applications", len(mergedList)))
	return mergedList
}
