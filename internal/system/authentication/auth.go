package authentication

import (
	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/api_keys/store"
	"github.com/wso2/identity-customer-data-service/internal/system/errors"
)

// ValidateAuthentication validates Authorization: ApiKey header from the HTTP request
func ValidateAuthentication(r *http.Request, event model.Event) (valid bool, error error) {
	token, err := extractAPIKey(r)
	if err != nil {
		return false, err
	}
	orgID, appID := fetchOrgAndApp(event)
	return validateAPIKey(token, orgID, appID)
}

func extractAPIKey(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedRequest.Code,
			Message:     errors.ErrUnAuthorizedRequest.Message,
			Description: errors.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "apikey" {
		log.Print("Invalid API key format")
		return "", errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedRequest.Code,
			Message:     errors.ErrUnAuthorizedRequest.Message,
			Description: errors.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
	}
	return parts[1], nil
}

func fetchOrgAndApp(event model.Event) (orgID, appID string) {
	// Extract values safely
	orgID = event.OrgId
	appID = event.AppId

	// Normalize default tenant
	if orgID == "carbon.super" {
		orgID = "-1234"
	}

	return orgID, appID
}

func validateAPIKey(apiKeyStr, orgID, appID string) (valid bool, error error) {
	if apiKeyStr == "" || appID == "" || orgID == "" {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "missing_fields",
			Message:     "Missing required fields",
			Description: "api_key, application_id, or org_id is missing",
		}, http.StatusBadRequest)
	}

	dbKey, err := store.GetAPIKey(apiKeyStr)
	if err != nil || dbKey == nil {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "invalid_api_key",
			Message:     "API key not found",
			Description: "Provided API key is not valid",
		}, http.StatusUnauthorized)
	}

	if dbKey.AppID != appID {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "mismatch_app_id",
			Message:     "App ID does not match",
			Description: "API key does not belong to this application",
		}, http.StatusUnauthorized)
	}

	if dbKey.OrgID != orgID {
		if (orgID == "carbon.super" && dbKey.OrgID == "-1234") ||
			(dbKey.OrgID == "carbon.super" && orgID == "-1234") {
			orgID = "carbon.super"
		} else {
			return false, errors.NewClientError(errors.ErrorMessage{
				Code:        "mismatch_org_id",
				Message:     "Org ID does not match",
				Description: "API key does not belong to this organization",
			}, http.StatusUnauthorized)
		}
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "mismatch_org_id",
			Message:     "Org ID does not match",
			Description: "API key does not belong to this organization",
		}, http.StatusUnauthorized)
	}

	if dbKey.State != "active" {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "revoked",
			Message:     "API key is not active",
			Description: "API key is revoked or inactive",
		}, http.StatusUnauthorized)
	}

	now := time.Now().UTC().Unix()
	if dbKey.ExpiresAt < now {
		return false, errors.NewClientError(errors.ErrorMessage{
			Code:        "expired",
			Message:     "API key has expired",
			Description: "API key expiration time has passed",
		}, http.StatusUnauthorized)
	}

	return true, nil
}
