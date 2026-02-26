package workers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	profileModel "github.com/wso2/identity-customer-data-service/internal/profile/model"
	schemaModel "github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// resolveFunc is the function called for each profile dequeued from the worker.
var resolveFunc func(profileModel.Profile)

// RegisterResolveFunc sets the async identity resolution function.
// Called once at server startup to inject the dependency from the identity_resolution package, avoiding an import cycle.
func RegisterResolveFunc(fn func(profileModel.Profile)) {
	resolveFunc = fn
}

var UnificationQueue chan profileModel.Profile

// pendingSet tracks profile IDs currently in the queue to prevent duplicate enqueues.
var pendingSet sync.Map

func StartProfileWorker() {

	UnificationQueue = make(chan profileModel.Profile, constants.DefaultQueueSize)

	go func() {
		logger := log.GetLogger()
		for profile := range UnificationQueue {
			pendingSet.Delete(profile.ProfileId)
			if resolveFunc != nil {
				resolveFunc(profile)
			} else {
				logger.Warn(fmt.Sprintf("ProfileWorker: no resolve function registered, skipping profile '%s'", profile.ProfileId))
			}
		}
	}()
}

func EnqueueProfileForProcessing(profile profileModel.Profile) {
	if UnificationQueue != nil {
		if _, alreadyQueued := pendingSet.LoadOrStore(profile.ProfileId, true); alreadyQueued {
			logger := log.GetLogger()
			logger.Info(fmt.Sprintf("ProfileWorker: profile '%s' already in queue, skipping duplicate enqueue", profile.ProfileId))
			return
		}
		UnificationQueue <- profile
	}
}

// ProfileWorkerQueue Define a struct that implements the EventQueue interface
type ProfileWorkerQueue struct{}

// Enqueue Implement the Enqueue method for ProfileWorkerQueue
func (q *ProfileWorkerQueue) Enqueue(profile profileModel.Profile) {
	EnqueueProfileForProcessing(profile)
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

		// Perform merge based on strategy
		mergedVal := MergeTraitValue(existingVal, newVal, rule.MergeStrategy, rule.ValueType, rule.MultiValued)

		if mergedVal == nil || mergedVal == "" {
			// keep it absent instead of "key": null
			continue
		}
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

	merged.OrgHandle = incomingProfile.OrgHandle // todo: need to
	merged.CreatedAt = existingProfile.CreatedAt // todo: need to decide on this too.
	merged.UpdatedAt = time.Now().UTC()

	return merged
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
		// todo:  We rely on the new value. But ideally we should define more precsise rules to merge the values.
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
			// Usually not multi-valued, but if so, allow combining
			a := toBoolSlice(existing)
			b := toBoolSlice(incoming)
			return combineUniqueBools(a, b)

		case "date_time", "datetime":
			a := toStringSlice(existing) // Assuming ISO date strings
			b := toStringSlice(incoming)
			return combineUniqueStrings(a, b)

		case "object":
			// Arrays of objects not merged by default: return incoming or append as-is
			a, okA := existing.([]interface{})
			b, okB := incoming.([]interface{})
			if okA && okB {
				return append(a, b...)
			}
			return incoming

		default:
			// Unknown or unsupported type — just return incoming
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
