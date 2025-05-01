package handlers

import (
	"github.com/google/uuid"
	"github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"github.com/wso2/identity-customer-data-service/pkg/service"
	"github.com/wso2/identity-customer-data-service/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AddUnificationRule handles adding a new rule
func (s Server) AddUnificationRule(c *gin.Context) {

	var rule models.UnificationRule
	if rule.RuleId == "" {
		rule.RuleId = uuid.NewString()
	}
	if err := c.ShouldBindJSON(&rule); err != nil {
		badReq := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrBadRequest.Code,
			Message:     errors.ErrBadRequest.Message,
			Description: err.Error(),
		}, http.StatusBadRequest)

		utils.HandleError(c, badReq)
		return
	}
	err := service.AddUnificationRule(rule)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rule) // Return the created rule in JSON format
}

// GetUnificationRules handles adding a new rule
func (s Server) GetUnificationRules(c *gin.Context) {

	rules, err := service.GetUnificationRules()
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, rules)
}

// GetUnificationRule Fetches a specific resolution rule.
func (s Server) GetUnificationRule(c *gin.Context, ruleId string) {

	rule, err := service.GetUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

// PatchUnificationRule applies partial updates to a unification rule.
func (s Server) PatchUnificationRule(c *gin.Context, ruleId string) {

	var updates bson.M
	if err := c.ShouldBindJSON(&updates); err != nil {
		badReq := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrBadRequest.Code,
			Message:     errors.ErrBadRequest.Message,
			Description: err.Error(),
		}, http.StatusBadRequest)

		utils.HandleError(c, badReq)
		return
	}

	err := service.PatchResolutionRule(ruleId, updates)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	rule, err := service.GetUnificationRule(ruleId)

	c.JSON(http.StatusOK, rule)
}

// DeleteUnificationRule removes a resolution rule.
func (s Server) DeleteUnificationRule(c *gin.Context, ruleId string) {

	err := service.DeleteUnificationRule(ruleId)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, "Unification rule deleted successfully")
}
