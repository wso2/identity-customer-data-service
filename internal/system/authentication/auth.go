package authentication

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/wso2/identity-customer-data-service/internal/system/cache"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/config"
)

var (
	tokenCache = cache.NewCache(15 * time.Minute)
)

// ValidateAuthentication validates Authorization: Bearer token from the HTTP request
func ValidateAuthentication(r *http.Request) (map[string]interface{}, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}

	claims, err := validateToken(token)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ErrUnAuthorizedRequest.Code,
			Message:     errors2.ErrUnAuthorizedRequest.Message,
			Description: errors2.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
		return "", clientError
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ErrUnAuthorizedRequest.Code,
			Message:     errors2.ErrUnAuthorizedRequest.Message,
			Description: errors2.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
		return "", clientError
	}
	return parts[1], nil
}

func validateToken(token string) (map[string]interface{}, error) {

	cachedClaims, found := tokenCache.Get(token)
	if found {
		if claims, ok := cachedClaims.(map[string]interface{}); ok {
			return claims, nil
		}
	}

	claims, err := introspectToken(token)
	if err != nil {
		return nil, err
	}

	active, ok := claims["active"].(bool)
	if !ok || !active {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ErrUnAuthorizedExpiryRequest.Code,
			Message:     errors2.ErrUnAuthorizedExpiryRequest.Message,
			Description: errors2.ErrUnAuthorizedExpiryRequest.Description,
		}, http.StatusUnauthorized)
		return nil, clientError
	}

	audiences, ok := claims["aud"].([]interface{})
	if !ok {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ErrInvalidAudience.Code,
			Message:     errors2.ErrInvalidAudience.Message,
			Description: errors2.ErrInvalidAudience.Description,
		}, http.StatusUnauthorized)
		return nil, clientError
	}

	hasCDS := false
	for _, aud := range audiences {
		if audStr, ok := aud.(string); ok && audStr == "iam-cds" {
			hasCDS = true
			break
		}
	}
	if !hasCDS {
		clientError := errors2.NewClientError(errors2.ErrorMessage{
			Code:        errors2.ErrInvalidAudience.Code,
			Message:     errors2.ErrInvalidAudience.Message,
			Description: errors2.ErrInvalidAudience.Description,
		}, http.StatusUnauthorized)
		return nil, clientError
	}

	tokenCache.Set(token, claims) // ✅ Store fresh introspection claims
	return claims, nil
}

func introspectToken(token string) (map[string]interface{}, error) {
	runtimeConfig := config.GetCDSRuntime().Config
	introspectionURL := runtimeConfig.AuthServer.IntrospectionEndPoint
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	data := url.Values{}
	data.Set("token", token)

	req, err := http.NewRequest("POST", introspectionURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileIntrospectingNewToken, err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(runtimeConfig.AuthServer.AdminUsername + ":" + runtimeConfig.AuthServer.AdminPassword))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileIntrospectingNewToken, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, errors2.NewServerError(errors2.ErrWhileIntrospectingNewToken, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors2.NewServerError(errors2.ErrWhileIntrospectingNewToken, err)
	}

	return result, nil
}

func GetTokenFromIS(applicationId string) (string, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Only for local/dev
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	runtimeConfig := config.GetCDSRuntime().Config
	endpoint := runtimeConfig.AuthServer.TokenEndpoint

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("tokenBindingId", applicationId)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", errors2.NewServerError(errors2.ErrWhileIssuingNewToken, err)
	}

	// Basic Auth Header (e.g., client_id:client_secret)
	encoded := base64.StdEncoding.EncodeToString([]byte(runtimeConfig.AuthServer.ClientID + ":" + runtimeConfig.AuthServer.ClientSecret))

	req.Header.Add("Authorization", "Basic "+encoded)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		log.Print("making request to token endpoint")
		return "", errors2.NewServerError(errors2.ErrWhileIssuingNewToken, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Print("response status code not ok")
		return "", errors2.NewServerError(errors2.ErrWhileIssuingNewToken, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Print("response body unmarshalled")
		return "", errors2.NewServerError(errors2.ErrWhileIssuingNewToken, err)
	}

	logger.Info("response body unmarshalled")
	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", errors2.NewServerError(errors2.ErrWhileIssuingNewToken, err)
	}
	logger.Info(fmt.Sprintf("New access token generated for application : '%s'", applicationId))
	return accessToken, nil
}

// Revoke token issued as write key
func RevokeToken(token string) error {

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	runtimeConfig := config.GetCDSRuntime().Config
	endpoint := runtimeConfig.AuthServer.RevocationEndpoint

	data := url.Values{}
	data.Set("token", token)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return errors2.NewServerError(errors2.ErrWhileRevokingToken, err)
	}

	// Basic Auth Header (same as token endpoint)
	encoded := base64.StdEncoding.EncodeToString([]byte(runtimeConfig.AuthServer.ClientID + ":" + runtimeConfig.AuthServer.ClientSecret))

	req.Header.Add("Authorization", "Basic "+encoded)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return errors2.NewServerError(errors2.ErrWhileRevokingToken, err)
	}
	defer resp.Body.Close()

	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return errors2.NewServerError(errors2.ErrWhileRevokingToken, err)
	}

	logger.Info("Token got revoked successfully")
	return nil
}
