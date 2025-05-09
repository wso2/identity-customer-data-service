package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/wso2/identity-customer-data-service/internal/constants"
	"github.com/wso2/identity-customer-data-service/internal/models"
	"github.com/wso2/identity-customer-data-service/internal/service"
	"github.com/wso2/identity-customer-data-service/internal/utils"
	"net/http"
)

// GetProfile handles profile retrieval requests
func (s Server) GetProfile(c *gin.Context, profileId string) {

	var profile *models.Profile
	var err error
	profile, err = service.GetProfile(profileId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profile)
}

// DeleteProfile handles profile deletion
func (s Server) DeleteProfile(c *gin.Context, profileId string) {
	err := service.DeleteProfile(profileId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetAllProfiles handles profile retrieval with and without filters
func (s Server) GetAllProfiles(c *gin.Context) {

	var profiles []models.Profile
	var err error
	// Build the filter from query params
	filter := c.QueryArray(constants.Filter) // Handles multiple filters
	if len(filter) > 0 {
		profiles, err = service.GetAllProfilesWithFilter(filter)
	} else {
		profiles, err = service.GetAllProfiles()
	}
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, profiles)
}
