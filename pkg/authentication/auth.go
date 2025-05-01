package authentication

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/wso2/identity-customer-data-service/pkg/cache"
	"github.com/wso2/identity-customer-data-service/pkg/constants"
	"github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/logger"
	"github.com/wso2/identity-customer-data-service/pkg/utils"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	tokenCache = cache.NewCache(15 * time.Minute)
)

// ValidateAuthentication validates Authorization: Bearer token from context
func ValidateAuthentication(c *gin.Context) (map[string]interface{}, error) {

	token, err := extractBearerToken(c)
	if err != nil {
		return nil, err
	}

	claims, err := validateToken(token)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func extractBearerToken(c *gin.Context) (string, error) {

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedRequest.Code,
			Message:     errors.ErrUnAuthorizedRequest.Message,
			Description: errors.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
		return "", clientError
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedRequest.Code,
			Message:     errors.ErrUnAuthorizedRequest.Message,
			Description: errors.ErrUnAuthorizedRequest.Description,
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
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedExpiryRequest.Code,
			Message:     errors.ErrUnAuthorizedExpiryRequest.Message,
			Description: errors.ErrUnAuthorizedExpiryRequest.Description,
		}, http.StatusUnauthorized)
		return nil, clientError
	}

	audiences, ok := claims["aud"].([]interface{})
	if !ok {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrInvalidAudience.Code,
			Message:     errors.ErrInvalidAudience.Message,
			Description: errors.ErrInvalidAudience.Description,
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
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrInvalidAudience.Code,
			Message:     errors.ErrInvalidAudience.Message,
			Description: errors.ErrInvalidAudience.Description,
		}, http.StatusUnauthorized)
		return nil, clientError
	}

	tokenCache.Set(token, claims) // ✅ Store fresh introspection claims
	return claims, nil
}

func introspectToken(token string) (map[string]interface{}, error) {

	introspectionURL := "https://localhost:9443/oauth2/introspect"
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
		return nil, errors.NewServerError(errors.ErrWhileIntrospectingNewToken, err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte("admin:admin"))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.NewServerError(errors.ErrWhileIntrospectingNewToken, err)

	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, errors.NewServerError(errors.ErrWhileIntrospectingNewToken, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, errors.NewServerError(errors.ErrWhileIntrospectingNewToken, err)
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

	endpoint, err := utils.BuildURL(constants.TokenEndpoint)

	if err != nil {
		return "", errors.NewServerError(errors.ErrWhileBuildingPath, err)
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("tokenBindingId", applicationId)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", errors.NewServerError(errors.ErrWhileIssuingNewToken, err)
	}

	// Basic Auth Header (e.g., client_id:client_secret)
	auth := "k06eyXqdJvoSBx_steWLWdCruBca:fjxoAQVCJlvTprKxZVd3tIl733fzWrvB5gJcKgqBBRYa" // Replace with actual client credentials if available
	encoded := base64.StdEncoding.EncodeToString([]byte(auth))

	req.Header.Add("Authorization", "Basic "+encoded)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		log.Print("making request to token endpoint")
		return "", errors.NewServerError(errors.ErrWhileIssuingNewToken, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Print("response status code not ok")
		return "", errors.NewServerError(errors.ErrWhileIssuingNewToken, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Print("response body unmarshalled")
		return "", errors.NewServerError(errors.ErrWhileIssuingNewToken, err)
	}

	logger.Info("response body unmarshalled")
	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", errors.NewServerError(errors.ErrWhileIssuingNewToken, err)
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

	endpoint, err := utils.BuildURL(constants.RevocationEndpoint)

	if err != nil {
		return errors.NewServerError(errors.ErrWhileBuildingPath, err)
	}

	data := url.Values{}
	data.Set("token", token)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return errors.NewServerError(errors.ErrWhileRevokingToken, err)
	}

	// Basic Auth Header (same as token endpoint)
	auth := "k06eyXqdJvoSBx_steWLWdCruBca:fjxoAQVCJlvTprKxZVd3tIl733fzWrvB5gJcKgqBBRYa" // Replace with actual client credentials if available
	encoded := base64.StdEncoding.EncodeToString([]byte(auth))

	req.Header.Add("Authorization", "Basic "+encoded)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return errors.NewServerError(errors.ErrWhileRevokingToken, err)
	}
	defer resp.Body.Close()

	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return errors.NewServerError(errors.ErrWhileRevokingToken, err)
	}

	logger.Info("Token got revoked successfully")
	return nil
}
