package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/database"
	erm "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/model"
	erp "github.com/wso2/identity-customer-data-service/internal/enrichment_rules/provider"
	"github.com/wso2/identity-customer-data-service/internal/enrichment_rules/store"
	model3 "github.com/wso2/identity-customer-data-service/internal/events/model"
	service2 "github.com/wso2/identity-customer-data-service/internal/events/service"
	model2 "github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/profile/service"
	repositories "github.com/wso2/identity-customer-data-service/internal/profile/store"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/model"
	"github.com/wso2/identity-customer-data-service/internal/unification_rules/provider"
)

var EnrichmentQueue chan model3.Event

func StartProfileWorker() {
	EnrichmentQueue = make(chan model3.Event, 1000)

	go func() {
		for event := range EnrichmentQueue {
			postgresDB := database.GetPostgresInstance()
			profileRepo := repositories.NewProfileRepository(postgresDB.DB)

			// Step 1: Enrich
			if err := EnrichProfile(event); err != nil {
				logger.Error(err, fmt.Sprintf("Failed to enrich profile %a with event %s ", event.ProfileId,
					event.EventId))
				continue
			}

			// Step 2: Unify
			profile, err := profileRepo.GetProfile(event.ProfileId)
			if err == nil && profile != nil {
				if _, err := unifyProfiles(*profile); err != nil {
					logger.Error(err, fmt.Sprintf("Failed to unify profile %s with event %s ", event.ProfileId,
						event.EventId))
				}
			}
		}
	}()
}

func EnqueueEventForProcessing(event model3.Event) {
	if EnrichmentQueue != nil {
		EnrichmentQueue <- event
	}
}

// Define a struct that implements the EventQueue interface
type ProfileWorkerQueue struct{}

// Implement the Enqueue method for ProfileWorkerQueue
func (q *ProfileWorkerQueue) Enqueue(event model3.Event) {
	EnqueueEventForProcessing(event)
}

// EnrichProfile extracts properties from events and enrich profile based on the enrichment rules
func EnrichProfile(event model3.Event) error {

	postgresDB := database.GetPostgresInstance() // Your method to get *sql.DB wrapped or raw
	profileRepo := repositories.NewProfileRepository(postgresDB.DB)
	profile, _ := service.WaitForProfile(event.ProfileId, 5, 100*time.Millisecond)

	if profile == nil {
		return fmt.Errorf("profile not found to enrich")
	}

	if profile.ProfileHierarchy != nil {
		if !profile.ProfileHierarchy.IsParent {
			// If the profile is a child, we need to enrich the parent profile
			profile, _ = profileRepo.GetProfile(profile.ProfileHierarchy.ParentProfileID)
		}
	}

	err := defaultInsertAppData(event, profile, profileRepo)
	if err != nil {
		return err
	}

	ruleProvider := erp.NewEnrichmentRuleProvider()
	ruleService := ruleProvider.GetEnrichmentRuleService()
	rules, _ := ruleService.GetEnrichmentRules()
	for _, rule := range rules {
		if strings.ToLower(rule.Trigger.EventType) != strings.ToLower(event.EventType) ||
			strings.ToLower(rule.Trigger.EventName) != strings.ToLower(event.EventName) {
			continue
		}

		// Step 2: Evaluate conditions
		if !service2.EvaluateConditions(event, rule.Trigger.Conditions) {
			continue
		}

		// Step 3: GetAPIKeyService value to assign
		var value interface{}
		if rule.ComputationMethod == "static" {
			value = rule.Value
		} else {
			// Basic "copy" computation
			switch strings.ToLower(rule.ComputationMethod) {
			case "extract":
				if rule.SourceField == "" {
					continue
				}
				value = service2.GetFieldFromEvent(event, rule.SourceField)
			case "count":
				// Since events are per profile - going back to child profile
				count, err := service2.CountEventsMatchingRule(event.ProfileId, rule.Trigger, rule.TimeRange)
				if err != nil {
					logger.Info("Failed to compute count for rule %s: %v", rule.RuleId, err)
					continue
				}
				value = count
			default:
				logger.Info("Unsupported computation: %s", rule.ComputationMethod)
				continue
			}
		}

		if value == nil {
			continue // skip if value couldn't be extracted
		}

		// Step 4: Apply merge strategy (existing value + new value)
		traitPath := strings.Split(rule.PropertyName, ".")
		if len(traitPath) == 0 {
			log.Printf("Invalid trait path: %s", rule.PropertyName)
			continue
		}

		namespace := traitPath[0]
		traitName := traitPath[1]
		fieldPath := fmt.Sprintf("%s.%s", namespace, traitName)
		if rule.ValueType != "" {
			value = parseValueForValueType(rule.ValueType, value)
		}
		update := bson.M{fieldPath: value}
		switch namespace {
		case "traits":
			err := profileRepo.UpsertTrait(profile.ProfileId, update)
			if err != nil {
				log.Println("Error updating personality data:", err)
			}
		case "identity_attributes":
			err := profileRepo.UpsertIdentityAttribute(profile.ProfileId, update)
			if err != nil {
				log.Println("Error updating identity data:", err)
			}
			continue
		case "application_data":
			err := profileRepo.UpsertAppDatum(profile.ProfileId, event.AppId, update)
			if err != nil {
				log.Println("Error updating application data:", err)
			}
			continue
		default:
			log.Printf("Unsupported trait namespace: %s", namespace)
			continue
		}
	}

	return nil
}

func defaultInsertAppData(event model3.Event, profile *model2.Profile, profileRepo *repositories.ProfileRepository) error {
	if event.Context == nil {
		return nil
	}

	rawDeviceID, ok := event.Context["device_id"]
	deviceID, isStr := rawDeviceID.(string)
	if !ok || !isStr || deviceID == "" {
		return nil
	}

	devices := model2.Devices{
		DeviceId: deviceID,
		LastUsed: event.EventTimestamp,
	}

	// Optional enrichments
	if os, ok := event.Context["os"].(string); ok {
		devices.Os = os
	}
	if browser, ok := event.Context["browser"].(string); ok {
		devices.Browser = browser
	}
	if version, ok := event.Context["browser_version"].(string); ok {
		devices.BrowserVersion = version
	}
	if ip, ok := event.Context["ip"].(string); ok {
		devices.Ip = ip
	}
	if deviceType, ok := event.Context["device_type"].(string); ok {
		devices.DeviceType = deviceType
	}

	// Determine which profile to enrich
	profileId := event.ProfileId
	if profile.ProfileHierarchy != nil && !profile.ProfileHierarchy.IsParent {
		profileId = profile.ProfileHierarchy.ParentProfileID
	}

	// Convert device list to generic format
	deviceList := []model2.Devices{devices}
	devicesJSON, err := json.Marshal(deviceList)
	if err != nil {
		return fmt.Errorf("failed to marshal device list: %w", err)
	}

	var deviceInterface interface{}
	if err := json.Unmarshal(devicesJSON, &deviceInterface); err != nil {
		return fmt.Errorf("failed to convert devices to generic interface: %w", err)
	}

	//  Fix: use namespaced key to signal top-level device field
	updates := map[string]interface{}{
		"application_data.devices": deviceInterface,
	}

	if err := profileRepo.UpsertAppDatum(profileId, event.AppId, updates); err != nil {
		return fmt.Errorf("failed to enrich application data: %v", err)
	}

	return nil
}

func unifyProfiles(newProfile model2.Profile) (*model2.Profile, error) {

	postgresDB := database.GetPostgresInstance()
	profileRepo := repositories.NewProfileRepository(postgresDB.DB)

	// Step 1: Fetch all unification rules
	ruleProvider := provider.NewUnificationRuleProvider()
	ruleService := ruleProvider.GetUnificationRuleService()
	unificationRules, err := ruleService.GetUnificationRules()
	if err != nil {
		return nil, errors.New("failed to fetch unification rules")
	}

	// 🔹 Step 2: Fetch all existing profiles from DB
	existingMasterProfiles, err := profileRepo.GetAllMasterProfilesExceptForCurrent(newProfile)
	if err != nil {
		return nil, errors.New("failed to fetch existing profiles")
	}

	sortRulesByPriority(unificationRules)
	// 🔹 Step 3: Loop through unification rules and compare profiles
	for _, rule := range unificationRules {

		for _, existingMasterProfile := range existingMasterProfiles {

			if existingMasterProfile.ProfileId == newProfile.ProfileHierarchy.ParentProfileID {
				// Skip if the existing master profile is the parent of the new profile
				return &newProfile, nil
			}

			if doesProfileMatch(existingMasterProfile, newProfile, rule) {

				existingMasterProfile.ProfileHierarchy.ChildProfiles, _ = profileRepo.FetchChildProfiles(existingMasterProfile.ProfileId)

				//  Merge the existing master to the old master of current
				postgresDB := database.GetPostgresInstance()
				schemaRepo := store.NewProfileSchemaRepository(postgresDB.DB)
				enrichmentRules, _ := schemaRepo.GetProfileEnrichmentRules()
				newMasterProfile := MergeProfiles(existingMasterProfile, newProfile, enrichmentRules)

				if len(existingMasterProfile.ProfileHierarchy.ChildProfiles) == 0 {
					newMasterProfile.ProfileId = uuid.New().String()
					childProfile1 := model2.ChildProfile{
						ChildProfileId: newProfile.ProfileId,
						RuleName:       rule.RuleName,
					}
					childProfile2 := model2.ChildProfile{
						ChildProfileId: existingMasterProfile.ProfileId,
						RuleName:       rule.RuleName,
					}
					newMasterProfile.ProfileHierarchy = &model2.ProfileHierarchy{
						IsParent:      true,
						ListProfile:   false,
						ChildProfiles: []model2.ChildProfile{childProfile1, childProfile2},
					}
					// creating and inserting the new master profile
					err := profileRepo.InsertProfile(newMasterProfile)
					if err != nil {
						return nil, err
					}

					err = profileRepo.UpdateParent(newMasterProfile, newProfile)
					err = profileRepo.UpdateParent(newMasterProfile, existingMasterProfile)

					children := []model2.ChildProfile{childProfile1, childProfile2}

					err = profileRepo.AddChildProfiles(newMasterProfile, children)

					if err != nil {
						return nil, err
					}

				} else if (len(existingMasterProfile.ProfileHierarchy.ChildProfiles) > 0) && existingMasterProfile.ProfileHierarchy.IsParent {
					newChild := model2.ChildProfile{
						ChildProfileId: newProfile.ProfileId,
						RuleName:       rule.RuleName,
					}
					children := []model2.ChildProfile{newChild}

					err = profileRepo.AddChildProfiles(newMasterProfile, children)
					err = profileRepo.UpdateParent(newMasterProfile, newProfile)
					if err != nil {
						return nil, err
					}
				}

				// Update ApplicationData
				for _, appCtx := range newMasterProfile.ApplicationData {
					err := profileRepo.InsertMergedMasterProfileAppData(newMasterProfile.ProfileId, appCtx)
					if err != nil {
						log.Println("Failed to update AppContext for:", appCtx.AppId, "Error:", err)
					}
				}

				// Update Traits
				if newMasterProfile.Traits != nil {
					err := profileRepo.InsertMergedMasterProfileTraitData(newMasterProfile.ProfileId, newMasterProfile.Traits)
					if err != nil {
						log.Println("Failed to update traits:", err)
					}
				}

				// Update Identity
				if newMasterProfile.IdentityAttributes != nil {
					err := profileRepo.MergeIdentityDataOfProfiles(newMasterProfile.ProfileId, newMasterProfile.IdentityAttributes)
					if err != nil {
						log.Println("Failed to update IdentityData:", err)
					}
				}

				return &newMasterProfile, nil

			}
		}
	}

	// No unification match found, return newProfile as-is
	return &newProfile, nil
}

func sortRulesByPriority(rules []model.UnificationRule) {
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
}

// MergeProfiles merges two profiles based on unification rules
func MergeProfiles(existingProfile model2.Profile, incomingProfile model2.Profile, enrichmentRules []erm.ProfileEnrichmentRule) model2.Profile {

	merged := existingProfile
	// todo: I doubt if this is fine.. we need to run through all to build a new profile
	for _, rule := range enrichmentRules {
		traitPath := strings.Split(rule.PropertyName, ".")
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
		case "application_data":
			merged.ApplicationData = mergeAppData(existingProfile.ApplicationData, incomingProfile.ApplicationData, enrichmentRules)
		}
	}

	return merged
}

// doesProfileMatch checks if two profiles have matching attributes based on a unification rule
func doesProfileMatch(existingProfile model2.Profile, newProfile model2.Profile, rule model.UnificationRule) bool {

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

// mergeStructFields merges non-zero fields from `src` into `dest`
func mergeStructFields(dest interface{}, src interface{}) {
	destVal := reflect.ValueOf(dest).Elem()
	srcVal := reflect.ValueOf(src).Elem()

	for i := 0; i < srcVal.NumField(); i++ {
		field := srcVal.Type().Field(i)
		srcField := srcVal.Field(i)
		destField := destVal.FieldByName(field.Name)

		// Skip if not settable or zero value
		if !destField.CanSet() || isZeroValue(srcField) {
			continue
		}

		// Handle slices: combine with deduplication
		if srcField.Kind() == reflect.Slice {
			merged := mergeSlices(destField.Interface(), srcField.Interface())
			destField.Set(reflect.ValueOf(merged))
			continue
		}

		// Simple overwrite
		destField.Set(srcField)
	}
}

// isZeroValue checks if a field is zero value (e.g. "", nil, 0, false)
func isZeroValue(v reflect.Value) bool {
	return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}

// mergeSlices merges two slices and removes duplicates
func mergeSlices(a, b interface{}) interface{} {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	existing := make(map[interface{}]bool)
	result := reflect.MakeSlice(aVal.Type(), 0, aVal.Len()+bVal.Len())

	// Helper to append unique values
	appendUnique := func(val reflect.Value) {
		if !existing[val.Interface()] {
			existing[val.Interface()] = true
			result = reflect.Append(result, val)
		}
	}

	for i := 0; i < aVal.Len(); i++ {
		appendUnique(aVal.Index(i))
	}
	for i := 0; i < bVal.Len(); i++ {
		appendUnique(bVal.Index(i))
	}

	return result.Interface()
}

// mergeDeviceLists merges devices, ensuring no duplicates based on `device_id`
func mergeDeviceLists(existingDevices, newDevices []model2.Devices) []model2.Devices {
	deviceMap := make(map[string]model2.Devices)

	for _, device := range existingDevices {
		if device.DeviceId != "" {
			deviceMap[device.DeviceId] = device
		}
	}

	for _, device := range newDevices {
		if device.DeviceId != "" {
			deviceMap[device.DeviceId] = device
		}
	}

	var mergedDevices []model2.Devices
	for _, device := range deviceMap {
		mergedDevices = append(mergedDevices, device)
	}
	return mergedDevices
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

func mergeAppData(existingAppData, incomingAppData []model2.ApplicationData, rules []erm.ProfileEnrichmentRule) []model2.ApplicationData {
	mergedMap := make(map[string]model2.ApplicationData)

	// Initialize with existingAppData
	for _, app := range existingAppData {
		mergedMap[app.AppId] = app
	}

	for _, newApp := range incomingAppData {
		existingApp, found := mergedMap[newApp.AppId]
		if !found {
			mergedMap[newApp.AppId] = newApp
			continue
		}

		// Merge devices
		existingApp.Devices = mergeDeviceLists(existingApp.Devices, newApp.Devices)

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
					if r.PropertyName == fmt.Sprintf("application_data.%s", key) {
						strategy = r.MergeStrategy
						valueType = r.ValueType
						break
					}
				}

				mergedVal := MergeTraitValue(existingVal, newVal, strategy, valueType)

				existingApp.AppSpecificData[key] = mergedVal

			}
		} else {
			log.Println("No app-specific data in incoming", "app_id", newApp.AppId)
		}

		mergedMap[newApp.AppId] = existingApp
	}

	var mergedList []model2.ApplicationData
	for appID, app := range mergedMap {
		log.Println("Final merged application data", "app_id", appID, "devices", app.Devices, "app_specific_data", app.AppSpecificData)
		mergedList = append(mergedList, app)
	}

	log.Println("Application data merge completed", "total_apps", len(mergedList))
	return mergedList
}
