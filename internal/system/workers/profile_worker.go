package workers

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	profileStore "github.com/wso2/identity-customer-data-service/internal/profile/store"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	schemaStore "github.com/wso2/identity-customer-data-service/internal/profile_schema/store"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/provider"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sort"
	"strconv"
	"strings"
	"time"
)

var UnificationQueue chan profileModel.Profile

func StartProfileWorker() {

	UnificationQueue = make(chan profileModel.Profile, 1000)

	go func() {
		for profile := range UnificationQueue {

			// Unify
			profile, err := profileStore.GetProfile(profile.ProfileId)
			if err == nil && profile != nil {
				unifyProfiles(*profile)
			}
		}
	}()
}

func EnqueueProfileForProcessing(profile profileModel.Profile) {
	if UnificationQueue != nil {
		UnificationQueue <- profile
	}
}

// Define a struct that implements the EventQueue interface
type ProfileWorkerQueue struct{}

// Implement the Enqueue method for ProfileWorkerQueue
func (q *ProfileWorkerQueue) Enqueue(profile profileModel.Profile) {
	EnqueueProfileForProcessing(profile)
}

// unifyProfiles unifies profiles based on unification rules
func unifyProfiles(newProfile profileModel.Profile) {

	// Step 1: Fetch all unification rules
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	unificationRules, err := ruleService.GetUnificationRules()
	logger := log.GetLogger()
	logger.Info(fmt.Sprintf("Beginning to evaluate unification for profile: %s", newProfile.ProfileId))
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch unification rules for unifying profile: %s",
			newProfile.ProfileId), log.Error(err))
	}

	// 🔹 Step 2: Fetch all existing profiles from DB
	existingMasterProfiles, err := profileStore.GetAllReferenceProfilesExceptForCurrent(newProfile)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch existing master profiles for unification of profile: %s",
			newProfile.ProfileId), log.Error(err))
	}

	sortRulesByPriority(unificationRules)
	// 🔹 Step 3: Loop through unification rules and compare profiles
	for _, rule := range unificationRules {

		for _, existingMasterProfile := range existingMasterProfiles {

			if existingMasterProfile.ProfileId == newProfile.ProfileStatus.ReferenceProfileId {
				// Skip if the existing master profile is the parent of the new profile
				return
			}

			if doesProfileMatch(existingMasterProfile, newProfile, rule) {
				logger.Info("Profiles has matched for unification rule: " + rule.RuleId)

				existingMasterProfile.ProfileStatus.References, _ = profileStore.FetchProfilesThatAreReferenced(existingMasterProfile.ProfileId)

				//  Merge the existing master to the old master of current
				schemaRules, _ := schemaStore.GetProfileSchemaAttributesForOrg(newProfile.TenantId)
				newMasterProfile := MergeProfiles(existingMasterProfile, newProfile, schemaRules)

				if len(existingMasterProfile.ProfileStatus.References) == 0 {
					newMasterProfile.ProfileId = uuid.New().String()
					newMasterProfile.Location = utils.BuildProfileLocation(newMasterProfile.TenantId, newMasterProfile.ProfileId)
					childProfile1 := profileModel.Reference{
						ProfileId: newProfile.ProfileId,
						Reason:    rule.RuleName,
					}
					childProfile2 := profileModel.Reference{
						ProfileId: existingMasterProfile.ProfileId,
						Reason:    rule.RuleName,
					}
					newMasterProfile.ProfileStatus = &profileModel.ProfileStatus{
						IsReferenceProfile: true,
						ListProfile:        false,
						References:         []profileModel.Reference{childProfile1, childProfile2},
					}

					// creating and inserting the new master profile
					// todo: for perm-temp - no need to insert

					err := profileStore.InsertProfile(newMasterProfile)
					if err != nil {
						logger.Error(fmt.Sprintf("Failed to insert master profile while unifying profile: %s",
							newProfile.ProfileId), log.Error(err))
						return
					}

					children := []profileModel.Reference{childProfile1, childProfile2}

					err = profileStore.UpdateProfileReferences(newMasterProfile, children)

					if err != nil {
						logger.Error(fmt.Sprintf("Failed to add child profiles to the master profile: %s",
							newMasterProfile.ProfileId), log.Error(err))
						return
					}

				} else if (len(existingMasterProfile.ProfileStatus.References) > 0) && existingMasterProfile.ProfileStatus.IsReferenceProfile {
					newChild := profileModel.Reference{
						ProfileId: newProfile.ProfileId,
						Reason:    rule.RuleName,
					}
					children := []profileModel.Reference{newChild}

					err = profileStore.UpdateProfileReferences(newMasterProfile, children)
					if err != nil {
						logger.Error(fmt.Sprintf("Failed to add child profiles to the master profile: %s",
							newProfile.ProfileId), log.Error(err))
						return
					}
					if err != nil {
						logger.Error(fmt.Sprintf("Failed to update master profile: %s while unifying profile: %s",
							newMasterProfile.ProfileId, newProfile.ProfileId), log.Error(err))
						return
					}
				}

				// Update ApplicationData
				for _, appCtx := range newMasterProfile.ApplicationData {
					err := profileStore.InsertMergedMasterProfileAppData(newMasterProfile.ProfileId, appCtx)
					if err != nil {
						logger.Error(fmt.Sprintf("Failed to update app data for master profile: %s while unifying profile: %s",
							newMasterProfile.ProfileId, newProfile.ProfileId), log.Error(err))
						return
					}
				}

				// Update Traits
				if newMasterProfile.Traits != nil {
					err := profileStore.InsertMergedMasterProfileTraitData(newMasterProfile.ProfileId, newMasterProfile.Traits)
					if err != nil {
						logger.Error(fmt.Sprintf("Failed to update traits for master profile: %s while unifying profile: %s",
							newMasterProfile.ProfileId, newProfile.ProfileId), log.Error(err))
						return
					}
				}

				// Update Identity
				if newMasterProfile.IdentityAttributes != nil {
					err := profileStore.MergeIdentityDataOfProfiles(newMasterProfile.ProfileId, newMasterProfile.IdentityAttributes)
					if err != nil {
						logger.Error(fmt.Sprintf("Failed to update IdentityData for master profile: %s while unifying profile: %s",
							newMasterProfile.ProfileId, newProfile.ProfileId), log.Error(err))
						return
					}
				}
			}
		}
	}
}

func sortRulesByPriority(rules []model.UnificationRule) {
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
}

// MergeProfiles merges two profiles based on unification rules
func MergeProfiles(existingProfile profileModel.Profile, incomingProfile profileModel.Profile, schemaRules []schemaModel.ProfileSchemaAttribute) profileModel.Profile {

	logger := log.GetLogger()
	logger.Info("Merging profiles, " + existingProfile.ProfileId + " and " + incomingProfile.ProfileId)
	merged := existingProfile
	// todo: I doubt if this is fine.. we need to run through all to build a new profile
	for _, rule := range schemaRules {
		traitPath := strings.Split(rule.AttributeName, ".")
		if len(traitPath) < 2 {
			continue
		}
		traitNamespace := traitPath[0]
		propertyName := traitPath[1]

		// Gather the fields for enrichment profiles
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

		// todo: FOR now when over-writing,existing is considered as the base profile

		// Perform merge based on strategy
		mergedVal := MergeTraitValue(existingVal, newVal, rule.MergeStrategy, rule.ValueType)

		// Apply merged result
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

	// Merge devices by default.
	merged.ApplicationData = mergeAppData(existingProfile.ApplicationData, incomingProfile.ApplicationData, schemaRules)

	if incomingProfile.UserId != "" {
		merged.UserId = incomingProfile.UserId // todo: need to decide on this as we are also focusing on perm-perm
	}
	if existingProfile.UserId != "" {
		merged.UserId = existingProfile.UserId // todo: need to decide on this as we are also focusing on perm-perm
	}

	merged.TenantId = incomingProfile.TenantId   // todo: need to decide on this too.
	merged.CreatedAt = existingProfile.CreatedAt // todo: need to decide on this too.
	merged.UpdatedAt = time.Now().Unix()

	return merged
}

// doesProfileMatch checks if two profiles have matching attributes based on a unification rule
func doesProfileMatch(existingProfile profileModel.Profile, newProfile profileModel.Profile, rule model.UnificationRule) bool {

	existingJSON, _ := json.Marshal(existingProfile)
	newJSON, _ := json.Marshal(newProfile)
	existingValues := extractFieldFromJSON(existingJSON, rule.Property)
	newValues := extractFieldFromJSON(newJSON, rule.Property)
	if checkForMatch(existingValues, newValues) {
		return true //  Match found
	}
	return false
}

// extractFieldFromJSON extracts a nested field from raw JSON (`[]byte`) without pre-converting to a map
func extractFieldFromJSON(jsonData []byte, fieldPath string) []interface{} {
	var jsonObj interface{}
	err := json.Unmarshal(jsonData, &jsonObj)
	if err != nil {
		return nil // Return nil if JSON parsing fails
	}

	// Navigate the JSON dynamically
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
		return list // Return extracted values from the list
	}

	return []interface{}{value} // Wrap a single value in a list
}

// checkForMatch checks if at least one value from `newProfile` exists in `existingProfile`
func checkForMatch(existingValues, newValues []interface{}) bool {
	existingSet := make(map[string]bool)
	for _, val := range existingValues {
		if str, ok := val.(string); ok {
			existingSet[str] = true
		}
	}

	// 🔹 Check if at least one value from `newValues` exists in `existingSet`
	for _, val := range newValues {
		if str, ok := val.(string); ok {
			if existingSet[str] {
				return true
			}
		}
	}
	return false
}

func parseValueForValueType(valueType string, raw interface{}) interface{} {
	strVal := fmt.Sprintf("%v", raw)

	switch strings.ToLower(valueType) {
	case "string":
		return strVal

	case "int":
		if i, err := strconv.Atoi(strVal); err == nil {
			return i
		}

	case "boolean":
		lower := strings.ToLower(strVal)
		return lower == "true" || lower == "1"

	case "arrayofstring":
		switch v := raw.(type) {
		case string:
			return []string{v}
		case []string:
			return v
		case []interface{}:
			var out []string
			for _, item := range v {
				out = append(out, fmt.Sprintf("%v", item))
			}
			return out
		default:
			return []string{strVal}
		}

	case "arrayofint":
		switch v := raw.(type) {
		case int:
			return []int{v}
		case []int:
			return v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return []int{i}
			}
		case []interface{}:
			var out []int
			for _, item := range v {
				if num, err := strconv.Atoi(fmt.Sprintf("%v", item)); err == nil {
					out = append(out, num)
				}
			}
			return out
		default:
			if i, err := strconv.Atoi(strVal); err == nil {
				return []int{i}
			}
		}
	}

	// Fallback: return as-is
	return raw
}

func MergeTraitValue(existing interface{}, incoming interface{}, strategy string, valueType string) interface{} {

	switch strings.ToLower(strategy) {
	case "overwrite":
		if incoming == nil {
			return existing
		}
		if incoming == "" {
			return existing
		}
		// todo:  We rely on the new value. But ideally we should define more precsise rules to merge the values.
		return incoming

	case "ignore":
		if existing != nil {
			return existing
		}
		return incoming

	case "combine":
		switch strings.ToLower(valueType) {
		case "arrayofint":
			return combineUniqueInts(toIntSlice(existing), toIntSlice(incoming))
		case "arrayofstring":
			existingArr := toStringSlice(existing)
			incomingArr := toStringSlice(incoming)
			return combineUniqueStrings(existingArr, incomingArr)
		default:
			return incoming
		}

	default:
		// fallback to overwrite
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

func mergeAppData(existingAppData, incomingAppData []profileModel.ApplicationData, rules []schemaModel.ProfileSchemaAttribute) []profileModel.ApplicationData {

	logger := log.GetLogger()
	mergedMap := make(map[string]profileModel.ApplicationData)

	// Initialize with existingAppData
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

		// Merge app-specific data using rule-based strategies
		if existingApp.AppSpecificData == nil {
			existingApp.AppSpecificData = map[string]interface{}{}
		}
		if newApp.AppSpecificData != nil {
			for key, newVal := range newApp.AppSpecificData {
				existingVal := existingApp.AppSpecificData[key]

				// Find merge strategy from enrichment rules
				strategy := ""
				valueType := ""

				for _, r := range rules {
					if r.AttributeName == fmt.Sprintf("application_data.%s", key) {
						strategy = r.MergeStrategy
						valueType = r.ValueType
						break
					}
				}

				mergedVal := MergeTraitValue(existingVal, newVal, strategy, valueType)

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
