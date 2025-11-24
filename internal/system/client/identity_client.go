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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/internal/profile_schema/model"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/log"
)

type IdentityClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewIdentityClient creates an IdentityClient with a TLS/mTLS-ready HTTP client.
func NewIdentityClient(cfg config.Config) *IdentityClient {
	baseHostPort := cfg.AuthServer.Host
	if cfg.AuthServer.Port != "" {
		baseHostPort = cfg.AuthServer.Host + ":" + cfg.AuthServer.Port
	}
	log.GetLogger().Info("Creating IdentityClient with base URL: " + baseHostPort)

	httpClient, err := newOutboundHTTPClient(cfg.TLS, cfg.AuthServer.Host)
	if err != nil {
		log.GetLogger().Error("Failed to create outbound HTTPS client for IS", log.Error(err))
		os.Exit(1)
	}

	return &IdentityClient{
		BaseURL:    baseHostPort,
		HTTPClient: httpClient,
	}
}

// Builds an HTTP client with TLS/mTLS configuration for outbound requests.
// Validates the server using CA, and optionally presents a client certificate if mTLS is enabled.
func newOutboundHTTPClient(tlsCfg config.TLSConfig, serverHostForSNI string) (*http.Client, error) {
	// Resolve cert dir to absolute to avoid CWD surprises
	certDir := tlsCfg.CertDir
	if certDir == "" {
		certDir = "/etc/certs"
	}
	if !filepath.IsAbs(certDir) {
		if abs, err := filepath.Abs(certDir); err == nil {
			certDir = abs
		}
	}

	// Root CA pool (optional but recommended)
	var rootCAs *x509.CertPool
	if tlsCfg.CACert != "" {
		caPath := filepath.Join(certDir, tlsCfg.CACert)
		caPEM, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca_cert at %s: %w", caPath, err)
		}
		rootCAs = x509.NewCertPool()
		if ok := rootCAs.AppendCertsFromPEM(caPEM); !ok {
			return nil, fmt.Errorf("failed to append ca_cert into CertPool: %s", caPath)
		}
	}

	// Client cert/key for mTLS (optional)
	var clientCerts []tls.Certificate
	if tlsCfg.MTLSEnabled {
		clientCrt := filepath.Join(certDir, tlsCfg.ClientCert)
		clientKey := filepath.Join(certDir, tlsCfg.ClientKey)
		pair, err := tls.LoadX509KeyPair(clientCrt, clientKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key (%s, %s): %w", clientCrt, clientKey, err)
		}
		clientCerts = []tls.Certificate{pair}
	}

	tcfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      rootCAs,          // if nil, system roots are used
		Certificates: clientCerts,      // empty if mTLS disabled
		ServerName:   serverHostForSNI, // ensure hostname verification (SNI)
	}

	tr := &http.Transport{
		TLSClientConfig:     tcfg,
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     60 * time.Second,
		MaxIdleConns:        100,
		MaxConnsPerHost:     100,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}, nil
}

// Fetch token using client_credentials grant
func (c *IdentityClient) fetchClientCredentialsToken(orgId string) (string, error) {

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "internal_application_mgt_view internal_claim_meta_view internal_user_mgt_list internal_user_mgt_view internal_claim_meta_view")

	authConfig := config.GetCDSRuntime().Config.AuthServer
	tokenEndpoint := "https://" + c.BaseURL + "/t/" + orgId + authConfig.TokenEndpoint
	logger := log.GetLogger()
	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to create token request for the organization:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errorMsg,
		}, err)
	}

	req.SetBasicAuth(authConfig.ClientID, authConfig.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to fetch token for the organization:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		// This is an internal communication. So for the clients of CDS, we treat this as a server error.
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errorMsg,
		}, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Token endpoint returned status %d: for the organization:%s", resp.StatusCode, orgId)
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errorMsg,
		}, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to read token response for the organization:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errorMsg,
		}, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		errorMsg := fmt.Sprintf("Failed to parse token response for the organization:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errorMsg,
		}, err)
	}
	token, ok := result["access_token"].(string)
	if !ok {
		errorMsg := fmt.Sprintf("Access token not found in response for the organization:%s", orgId)
		return "", errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.TOKEN_FETCH_FAILED.Code,
			Message:     errors2.TOKEN_FETCH_FAILED.Message,
			Description: errorMsg,
		}, err)
	}

	return token, nil
}

// IntrospectToken introspects an opaque token using the introspection endpoint.
func (c *IdentityClient) IntrospectToken(orgId, token string) (map[string]interface{}, error) {

	form := url.Values{}
	form.Set("token", token)

	authConfig := config.GetCDSRuntime().Config.AuthServer
	introspectionEndpoint := "https://" + c.BaseURL + "/t/" + orgId + authConfig.IntrospectionEndPoint
	log.GetLogger().Info("Introspecting token at endpoint: " + introspectionEndpoint)

	req, err := http.NewRequest("POST", introspectionEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(authConfig.AdminUsername, authConfig.AdminPassword)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	logger := log.GetLogger()
	if err != nil {
		errorMsg := "Failed to introspect token"
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "TOKEN_INTROSPECTION_FAILED",
			Message:     "Unable to introspect access token",
			Description: errorMsg,
		}, http.StatusUnauthorized)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("Introspection endpoint returned status %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, errors2.NewClientError(errors2.ErrorMessage{
			Code:        "TOKEN_INTROSPECTION_INVALID_RESPONSE",
			Message:     "Token introspection failed",
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

func (c *IdentityClient) GetProfileSchema(orgId string) ([]model.ProfileSchemaAttribute, error) {

	logger := log.GetLogger()
	localClaimsMap, err := c.GetLocalClaimsMap(orgId)
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to fetch local claims for the organization:%s", orgId), log.Error(err))
		return nil, err
	}

	dialects, err := c.GetAllDialects(orgId)
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to fetch dialects for the organization:%s", orgId), log.Error(err))
		return nil, err
	}
	logger.Info(fmt.Sprintf("Fetched %d dialects for the organization:%s", len(dialects), orgId))
	var result []model.ProfileSchemaAttribute
	for _, dialect := range dialects {
		logger.Info("Processing dialect", log.String("dialectURI", fmt.Sprintf("%v", dialect["dialectURI"])))
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

		claims, err := c.GetClaimsByDialect(dialectID, orgId)
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
				if parent == "identity_attributes.emailaddress" {
					logger.Debug(fmt.Sprintf("Skip deriving complex parent attribute: '%s'", parent))
					continue // Skip as this has a separate attribute configuration
				}
				dialect := parentDialects[parent]
				if dialect == "" {
					dialect = "urn:synthetic" // fallback
				}
				logger.Warn(fmt.Sprintf("Adding synthetic parent attribute: %s", parent))
				result = append(result, model.ProfileSchemaAttribute{
					OrgId:         orgId,
					AttributeId:   uuid.New().String(),
					AttributeName: parent,
					ValueType:     constants.ComplexDataType,
					MergeStrategy: constants.MergeStrategyOverwrite,
					Mutability:    constants.MutabilityReadWrite,
					SubAttributes: subs,
					SCIMDialect:   dialect, // mark as generated
				})
			}
		}
	}

	logger.Info(fmt.Sprintf("Successfully built %d profile schema attributes for org: %s", len(result), orgId))
	return result, nil
}

func (c *IdentityClient) GetAllDialects(orgId string) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("https://%s/t/%s/api/server/v1/claim-dialects", c.BaseURL, orgId)
	req, _ := http.NewRequest("GET", endpoint, nil)
	logger := log.GetLogger()
	token, err := c.fetchClientCredentialsToken(orgId)
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to get token for fetching the all dialects of the organization:%s",
			orgId), log.Error(err))
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		// This is an internal communication. So for the clients of CDS, we treat this as a server error.
		errorMsg := fmt.Sprintf("Failed to fetch all dialects for the organization:%s", orgId)
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_SCIM_DIALECTS.Code,
			Message:     errors2.GET_SCIM_DIALECTS.Message,
			Description: errorMsg,
		}, err)
	}
	defer resp.Body.Close()

	var dialects []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &dialects)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to parse dialects response for the organization:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_SCIM_DIALECTS.Code,
			Message:     errors2.GET_SCIM_DIALECTS.Message,
			Description: errorMsg,
		}, err)
	}
	return dialects, err
}

func (c *IdentityClient) GetClaimsByDialect(dialectID, orgId string) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("https://%s/t/%s/api/server/v1/claim-dialects/%s/claims", c.BaseURL, orgId, dialectID)
	req, _ := http.NewRequest("GET", endpoint, nil)
	token, err := c.fetchClientCredentialsToken(orgId)
	logger := log.GetLogger()
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to get token for fetching the claims of dialectID:%s of the organization:%s", dialectID, orgId), log.Error(err))
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch claims for dialectID:%s of the organization:%s", dialectID, orgId)
		logger.Debug(errMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_DIALECT_CLAIMS.Code,
			Message:     errors2.GET_DIALECT_CLAIMS.Message,
			Description: errMsg,
		}, err)
	}
	defer resp.Body.Close()

	var claims []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &claims)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to parse claims response for dialectID:%s of the organization:%s", dialectID, orgId)
		logger.Debug(errMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_DIALECT_CLAIMS.Code,
			Message:     errors2.GET_DIALECT_CLAIMS.Message,
			Description: errMsg,
		}, err)
	}
	return claims, err
}

func (c *IdentityClient) GetLocalClaimsMap(orgId string) (map[string]map[string]interface{}, error) {

	endpoint := fmt.Sprintf("https://%s/t/%s/api/server/v1/claim-dialects/local/claims", c.BaseURL, orgId)
	logger := log.GetLogger()
	logger.Info("Fetching local claims from endpoint: " + endpoint)
	req, _ := http.NewRequest("GET", endpoint, nil)
	token, err := c.fetchClientCredentialsToken(orgId)
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to get token for fetching the local claims of the organization:%s",
			orgId), log.Error(err))
		return nil, err
	}
	logger.Info("Fetching local claims from token: " + token)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to fetch local claims for the organization:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_LOCAL_CLAIMS_FAILED.Code,
			Message:     errors2.GET_LOCAL_CLAIMS_FAILED.Message,
			Description: errorMsg,
		}, err)
	}
	defer resp.Body.Close()

	var claims []map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		errorMsg := fmt.Sprintf("Failed to fetch local claims, status code: %d, response: %s", resp.StatusCode, string(body))
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_LOCAL_CLAIMS_FAILED.Code,
			Message:     errors2.GET_LOCAL_CLAIMS_FAILED.Message,
			Description: errorMsg,
		}, err)
	}
	err = json.Unmarshal(body, &claims)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to parse local claims response for the organization:%s", orgId)
		logger.Debug(errorMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_LOCAL_CLAIMS_FAILED.Code,
			Message:     errors2.GET_LOCAL_CLAIMS_FAILED.Message,
			Description: errorMsg,
		}, err)
	}

	// Build map using claimURI
	claimMap := make(map[string]map[string]interface{})
	for _, claim := range claims {
		uri := fmt.Sprintf("%v", claim["claimURI"])
		claimMap[uri] = claim
	}
	return claimMap, nil
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
	valueType := "string" // Default
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

	// Override to complex if it has sub-attributes
	if len(subAttrs) > 0 {
		valueType = "complex"
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
func (c *IdentityClient) GetSCIMUser(orgId, userId string) (map[string]interface{}, error) {

	endpoint := fmt.Sprintf("https://%s/t/%s/scim2/Users/%s", c.BaseURL, orgId, userId)
	req, _ := http.NewRequest("GET", endpoint, nil)
	token, err := c.fetchClientCredentialsToken(orgId)
	logger := log.GetLogger()
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to get token for fetching the SCIM user:%s of the organization:%s",
			userId, orgId), log.Error(err))
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch SCIM user:%s of the organization:%s", userId, orgId)
		logger.Debug(errMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_SCIM_USER_FAILED.Code,
			Message:     errors2.GET_SCIM_USER_FAILED.Message,
			Description: errMsg,
		}, err)
	}
	defer resp.Body.Close()

	var user map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &user); err != nil {
		errMsg := fmt.Sprintf("Failed to parse SCIM user response for user:%s of the organization:%s", userId, orgId)
		logger.Debug(errMsg, log.Error(err))
		return nil, errors2.NewServerError(errors2.ErrorMessage{
			Code:        errors2.GET_SCIM_USER_FAILED.Code,
			Message:     errors2.GET_SCIM_USER_FAILED.Message,
			Description: errMsg,
		}, err)
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
