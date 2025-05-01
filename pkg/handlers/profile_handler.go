package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/wso2/identity-customer-data-service/pkg/constants"
	"github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"github.com/wso2/identity-customer-data-service/pkg/service"
	"github.com/wso2/identity-customer-data-service/pkg/utils"
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

// CreateEnrichmentRule handles creating new profile enrichment rule
func (s Server) CreateEnrichmentRule(c *gin.Context) {

	var rules models.ProfileEnrichmentRule
	if err := c.ShouldBindJSON(&rules); err != nil {
		clientError := errors.NewClientErrorWithoutCode(errors.ErrorMessage{
			Code:        errors.ErrBadRequest.Code,
			Message:     errors.ErrBadRequest.Message,
			Description: err.Error(),
		})
		c.JSON(http.StatusBadRequest, clientError)
		return
	}

	err := service.AddEnrichmentRule(rules)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rules)
}

// GetEnrichmentRules handles retrieve of all rules with or without filters
func (s Server) GetEnrichmentRules(c *gin.Context) {

	filters := c.QueryArray(constants.Filter) // Handles multiple `filter=...` parameters

	if len(filters) > 0 {
		rules, err := service.GetEnrichmentRulesByFilter(filters)
		if err != nil {
			utils.HandleError(c, err)
			return
		}
		c.JSON(http.StatusOK, rules)
		return
	}

	// fallback: all rules
	rules, err := service.GetEnrichmentRules()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, rules)
}

// GetEnrichmentRule handles retrieivng a specific rule
func (s Server) GetEnrichmentRule(c *gin.Context, ruleId string) {
	rule, err := service.GetEnrichmentRule(ruleId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (s Server) PutEnrichmentRule(c *gin.Context, ruleId string) {
	//TODO update the implementation
	var rules models.ProfileEnrichmentRule

	// fetch and validate if it exists already

	if err := c.ShouldBindJSON(&rules); err != nil {
		badReq := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrBadRequest.Code,
			Message:     errors.ErrBadRequest.Message,
			Description: err.Error(),
		}, http.StatusBadRequest)
		utils.HandleError(c, badReq)
	}
	err := service.PutEnrichmentRule(rules)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, rules)
}

// DeleteEnrichmentRule handles DELETE /unification_rules/:rule_name
func (s Server) DeleteEnrichmentRule(c *gin.Context, ruleId string) {
	if ruleId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_name is required"})
		return
	}

	err := service.DeleteEnrichmentRule(ruleId)
	if err != nil {
		utils.HandleError(c, err)
	}
	c.JSON(http.StatusNoContent, nil)
}
