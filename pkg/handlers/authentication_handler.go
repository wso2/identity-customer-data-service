package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/wso2/identity-customer-data-service/pkg/authentication"
	"github.com/wso2/identity-customer-data-service/pkg/utils"
	"net/http"
)

func (s Server) GetWriteKey(c *gin.Context, application_id string) {
	// Step 1: Get existing token if needed (for now assume no previous token available)
	// If you have a DB or cache, fetch the existing token here.
	existingToken, _ := authentication.GetTokenFromIS(application_id)

	// Step 2: If token exists, revoke it first as this would be re-generating a new one
	if existingToken != "" {
		err := authentication.RevokeToken(existingToken)
		if err != nil {
			utils.HandleError(c, err)
			return
		}
	}

	// Step 3: Get a new token
	newToken, err := authentication.GetTokenFromIS(application_id)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"write_key": newToken})
}
