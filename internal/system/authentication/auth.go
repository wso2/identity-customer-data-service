package authentication

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2/identity-customer-data-service/internal/system/cache"
	errors2 "github.com/wso2/identity-customer-data-service/internal/system/errors"
	"net/http"
	"strings"
	"time"
)

var (
	tokenCache       = cache.NewCache(15 * time.Minute)
	expectedAudience = "iam-cds"
)

// ValidateAuthentication validates Authorization: Bearer token from the HTTP request
func ValidateAuthentication(r *http.Request) (bool, error) {
	token, err := extractBearerToken(r)
	if err != nil {
		return false, err
	}
	return validateToken(token)
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", unauthorizedError()
	}
	return strings.TrimPrefix(authHeader, "Bearer "), nil
}

func validateToken(token string) (bool, error) {
	//  Try cache
	if cached, found := tokenCache.Get(token); found {
		if claims, ok := cached.(map[string]interface{}); ok {
			if valid := validateClaims(claims); valid {
				return true, nil
			}
		}
	}

	//  For now assume JWT — parse claims
	claims, err := ParseJWTClaims(token)
	if err != nil {
		return false, unauthorizedError()
	}

	//  Validate claims
	if !validateClaims(claims) {
		return false, unauthorizedError()
	}

	// ⏺️ Store in cache
	tokenCache.Set(token, claims)

	return true, nil
}

// parseJWTClaims parses claims from a JWT without verifying the signature
func ParseJWTClaims(tokenString string) (map[string]interface{}, error) {
	claims := jwt.MapClaims{}
	_, _, err := new(jwt.Parser).ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// validateClaims ensures the token has `active: true` and the expected audience
func validateClaims(claims map[string]interface{}) bool {

	expRaw, ok := claims["exp"]
	if !ok {
		return false
	}
	expFloat, ok := expRaw.(float64)
	if !ok {
		return false
	}
	expUnix := int64(expFloat)
	currentTime := time.Now().Unix()
	if expUnix < currentTime {
		return false
	}

	audRaw, ok := claims["aud"]
	if !ok {
		return false
	}

	var audList []string
	switch aud := audRaw.(type) {
	case []interface{}:
		for _, a := range aud {
			if s, ok := a.(string); ok {
				audList = append(audList, s)
			}
		}
	case string:
		audList = append(audList, aud)
	}

	for _, aud := range audList {
		if aud == expectedAudience {
			return true
		}
	}
	return false
}

// GetCachedClaims returns claims from cache if available
func GetCachedClaims(token string) (map[string]interface{}, bool) {
	cached, found := tokenCache.Get(token)
	if !found {
		return nil, false
	}
	claims, ok := cached.(map[string]interface{})
	return claims, ok
}

func unauthorizedError() error {
	return errors2.NewClientError(errors2.ErrorMessage{
		Code:        errors2.ErrUnAuthorizedRequest.Code,
		Message:     errors2.ErrUnAuthorizedRequest.Message,
		Description: errors2.ErrUnAuthorizedRequest.Description,
	}, http.StatusUnauthorized)
}
