/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com).
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

package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

type IdentityClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

// Create new client and fetch token
func NewIdentityClient(cfg config.Config) *IdentityClient {

	client := &IdentityClient{
		BaseURL: cfg.AuthServer.Host + ":" + cfg.AuthServer.Port,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // ⚠️ Dev use only!
			},
		},
	}

	// Fetch token at startup
	tokenResp, err := client.fetchClientCredentialsToken()
	if err != nil {
		log.GetLogger().Error("Failed to fetch access token", log.Error(err))
		return client // Token will be empty
	}

	if accessToken, ok := tokenResp["access_token"].(string); ok {
		client.Token = accessToken
	} else {
		log.GetLogger().Error("Access token not found in token response")
	}

	return client
}

// Fetch token using client_credentials grant
func (c *IdentityClient) fetchClientCredentialsToken() (map[string]interface{}, error) {

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "internal_application_mgt_view internal_claim_meta_view internal_user_mgt_list internal_user_mgt_view internal_claim_meta_view") // Optional: set required scopes

	authConfig := config.GetCDSRuntime().Config.AuthServer
	tokenEndpoint := "https://" + authConfig.Host + ":" + authConfig.Port + "/" + authConfig.TokenEndpoint

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(authConfig.ClientID, authConfig.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to fetch token from identity server"
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "TOKEN_FETCH_FAILED",
			Message:     "Unable to get access token",
			Description: errorMsg,
		}, http.StatusUnauthorized)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("Token endpoint returned status %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "TOKEN_INVALID_RESPONSE",
			Message:     "Token fetch failed",
			Description: errorMsg,
		}, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *IdentityClient) GetProfileSchema() ([]model.ProfileSchemaAttribute, error) {
	orgId := "carbon.super"
	logger := log.GetLogger()

	localClaimsMap, err := c.GetLocalClaimsMap()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch local claims: %w", err)
	}

	dialects, err := c.GetAllDialects()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch dialects: %w", err)
	}
	log.GetLogger().Info("Fetched dialects", log.Int("count", len(dialects)))
	var result []model.ProfileSchemaAttribute
	for _, dialect := range dialects {
		log.GetLogger().Info("Processing dialect", log.String("dialectURI", fmt.Sprintf("%v", dialect["dialectURI"])))
		dialectURI := fmt.Sprintf("%v", dialect["dialectURI"])
		dialectID := fmt.Sprintf("%v", dialect["id"])

		// Only consider SCIM-based dialects
		if !(strings.HasPrefix(dialectURI, "urn:scim:") || strings.HasPrefix(dialectURI, "urn:ietf:")) {
			continue
		}
		// Skip SCIM core v1 dialect explicitly
		if dialectURI == "urn:scim:schemas:core:1.0" {
			continue
		}

		claims, err := c.GetClaimsByDialect(dialectID)
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to fetch claims for dialect %s", dialectURI))
			continue
		}

		existingAttrs := map[string]bool{}
		pendingParents := map[string][]model.SubAttribute{}
		parentDialects := map[string]string{}

		for _, scimClaim := range claims {
			localURI := fmt.Sprintf("%v", scimClaim["mappedLocalClaimURI"])
			localClaim, ok := localClaimsMap[localURI]
			log.GetLogger().Info("Processing SCIM claim", log.String("claimURI", fmt.Sprintf("%v", scimClaim["claimURI"])))
			log.GetLogger().Info("Processing local claim", log.String("localURI", localURI))
			if !ok {
				continue
			}

			attr, subAttr, parent := ConvertSCIMClaimWithLocal(scimClaim, localClaim, claims, orgId, dialectURI)
			result = append(result, attr)
			existingAttrs[attr.AttributeName] = true

			if subAttr != nil {
				parentKey := parent
				pendingParents[parentKey] = append(pendingParents[parentKey], *subAttr)

				// Store dialect only if it's not already set
				if _, exists := parentDialects[parentKey]; !exists {
					parentDialects[parentKey] = attr.SCIMDialect
				}
			}
		}

		// Add synthetic parent objects if missing
		for parent, subs := range pendingParents {
			if !existingAttrs[parent] {
				dialect := parentDialects[parent]
				if dialect == "" {
					dialect = "urn:synthetic" // fallback
				}
				result = append(result, model.ProfileSchemaAttribute{
					OrgId:         orgId,
					AttributeId:   uuid.New().String(),
					AttributeName: parent,
					ValueType:     "object",
					MergeStrategy: "overwrite",
					Mutability:    "readWrite",
					SubAttributes: subs,
					UpdatedAt:     0,
					SCIMDialect:   dialect, // mark as generated
				})
			}
		}
	}

	logger.Info(fmt.Sprintf("Successfully built %d profile schema attributes for org: %s", len(result), orgId))
	return result, nil
}

func (c *IdentityClient) GetAllDialects() ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("https://%s/api/server/v1/claim-dialects", c.BaseURL)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var dialects []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &dialects)
	return dialects, err
}

func (c *IdentityClient) GetClaimsByDialect(dialectID string) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("https://%s/api/server/v1/claim-dialects/%s/claims", c.BaseURL, dialectID)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var claims []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &claims)
	return claims, err
}

func (c *IdentityClient) GetLocalClaimsMap() (map[string]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("https://%s/api/server/v1/claim-dialects/local/claims", c.BaseURL)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var claims []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &claims)

	// Build map using claimURI
	claimMap := make(map[string]map[string]interface{})
	for _, claim := range claims {
		uri := fmt.Sprintf("%v", claim["claimURI"])
		claimMap[uri] = claim
	}
	return claimMap, err
}

func extractClaimKeyFromLocalURI(localURI string) string {
	parts := strings.Split(localURI, "/")
	return parts[len(parts)-1]
}

func ConvertSCIMClaimWithLocal(
	scim map[string]interface{},
	local map[string]interface{},
	allClaims []map[string]interface{},
	orgId, dialectURI string,
) (model.ProfileSchemaAttribute, *model.SubAttribute, string) {

	claimURI := fmt.Sprintf("%v", scim["claimURI"])
	localURI := fmt.Sprintf("%v", scim["mappedLocalClaimURI"])
	attrKey := extractClaimKeyFromLocalURI(localURI)

	readOnly := false
	multiValued := false
	if val, ok := local["readOnly"].(bool); ok {
		readOnly = val
	}
	valueType := "text" // Default
	if val, ok := local["dataType"].(string); ok {
		valueType = val
	}
	if props, ok := local["properties"].([]interface{}); ok {
		for _, p := range props {
			prop := p.(map[string]interface{})
			if prop["key"] == "multiValued" && prop["value"] == "true" {
				multiValued = true
			}
		}
	}

	// Handle canonicalValues
	var canonicalValues []model.CanonicalValue
	if vals, ok := local["canonicalValues"].([]interface{}); ok {
		for _, val := range vals {
			if valMap, ok := val.(map[string]interface{}); ok {
				canonical := model.CanonicalValue{}
				if v, ok := valMap["value"].(string); ok {
					canonical.Value = v
				}
				if l, ok := valMap["label"].(string); ok {
					canonical.Label = l
				}
				canonicalValues = append(canonicalValues, canonical)
			}
		}
	}

	// Find sub-attributes for the current attribute (if it's a parent)
	var subAttrs []model.SubAttribute
	for _, otherClaim := range allClaims {
		otherURI := fmt.Sprintf("%v", otherClaim["claimURI"])

		if strings.HasPrefix(otherURI, claimURI+".") {
			mappedLocalURI := fmt.Sprintf("%v", otherClaim["mappedLocalClaimURI"])

			// Ensure mapped local URI is truly nested under the current local URI
			if strings.HasPrefix(mappedLocalURI, localURI+".") {
				subAttrKey := extractClaimKeyFromLocalURI(mappedLocalURI)

				if strings.HasPrefix(subAttrKey, attrKey+".") {
					subAttrKey = strings.TrimPrefix(subAttrKey, attrKey+".")
				}

				subAttrs = append(subAttrs, model.SubAttribute{
					AttributeId:   fmt.Sprintf("%v", uuid.New().String()),
					AttributeName: "identity_attributes." + attrKey + "." + subAttrKey,
				})
			}
		}
	}

	fullAttrName := "identity_attributes." + attrKey

	// Check if this is a sub-attribute (i.e., contains a dot after the prefix)
	if strings.Contains(attrKey, ".") {
		parentAttrName := "identity_attributes." + strings.Split(attrKey, ".")[0]
		subAttr := model.SubAttribute{
			AttributeId:   fmt.Sprintf("%v", uuid.New().String()),
			AttributeName: fullAttrName,
		}
		return model.ProfileSchemaAttribute{
			OrgId:           orgId,
			AttributeId:     subAttr.AttributeId,
			AttributeName:   fullAttrName,
			ValueType:       valueType,
			MergeStrategy:   "overwrite",
			Mutability:      ifThenElse(readOnly, "readOnly", "readWrite"),
			MultiValued:     multiValued,
			CanonicalValues: canonicalValues,
			SubAttributes:   nil,
			UpdatedAt:       0,
			SCIMDialect:     dialectURI,
		}, &subAttr, parentAttrName
	}

	// It's a top-level or parent attribute
	return model.ProfileSchemaAttribute{
		OrgId:           orgId,
		AttributeId:     fmt.Sprintf("%v", uuid.New().String()),
		AttributeName:   fullAttrName,
		ValueType:       valueType,
		MergeStrategy:   "overwrite",
		Mutability:      ifThenElse(readOnly, "readOnly", "readWrite"),
		MultiValued:     multiValued,
		CanonicalValues: canonicalValues,
		SubAttributes:   subAttrs,
		UpdatedAt:       0,
		SCIMDialect:     dialectURI,
	}, nil, ""
}

func ifThenElse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// GetSCIMUser fetches a SCIM user by ID
func (c *IdentityClient) GetSCIMUser(userId string) (map[string]interface{}, error) {

	endpoint := fmt.Sprintf("https://%s/scim2/Users/%s", c.BaseURL, userId)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}

	return flattenSCIMClaims(user), nil
}

func flattenSCIMClaims(user map[string]interface{}) map[string]interface{} {
	flat := make(map[string]interface{})

	for key, val := range user {
		switch v := val.(type) {
		case map[string]interface{}:
			for subKey, subVal := range v {
				flat[key+"."+subKey] = subVal
			}
		default:
			flat[key] = val
		}
	}

	return flat
}
