package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/wso2/identity-customer-data-service/pkg/authentication"
	"github.com/wso2/identity-customer-data-service/pkg/errors"
	"github.com/wso2/identity-customer-data-service/pkg/models"
	"github.com/wso2/identity-customer-data-service/pkg/service"
	"github.com/wso2/identity-customer-data-service/pkg/utils"
	"go.mongodb.org/mongo-driver/bson"
	"net/http"
	"strconv"
	"time"
)

// AddEvent handles adding a single event
func (s Server) AddEvent(c *gin.Context) {

	if _, err := authentication.ValidateAuthentication(c); err != nil {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrUnAuthorizedRequest.Code,
			Message:     errors.ErrUnAuthorizedRequest.Message,
			Description: errors.ErrUnAuthorizedRequest.Description,
		}, http.StatusUnauthorized)
		c.JSON(http.StatusUnauthorized, clientError)
		return
	}

	var event models.Event

	if err := c.ShouldBindJSON(&event); err != nil {
		clientError := errors.NewClientError(errors.ErrorMessage{
			Code:        errors.ErrBadRequest.Code,
			Message:     errors.ErrBadRequest.Message,
			Description: err.Error(),
		}, http.StatusBadRequest)
		c.JSON(http.StatusBadRequest, clientError)
		return
	}

	if err := service.AddEvents(event); err != nil {
		utils.HandleError(c, err)
		return
	}

	c.Status(http.StatusAccepted)
}

// GetEvent fetches a specific event
func (s Server) GetEvent(c *gin.Context, eventId string) {
	events, err := service.GetEvent(eventId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

// TODO remove
func (s Server) GetEvents(c *gin.Context) {
	rawFilters := c.QueryArray("filter")

	// Step 2: Parse optional time range
	var timeFilter bson.M
	if timeStr := c.Query("time_range"); timeStr != "" {
		durationSec, _ := strconv.Atoi(timeStr)       // parse string to int
		currentTime := time.Now().UTC().Unix()        // current time in seconds
		startTime := currentTime - int64(durationSec) // assuming value is in minutes
		timeFilter = bson.M{
			"event_timestamp": bson.M{"$gte": startTime},
		}
	}

	// Step 3: Fetch events with filter strings
	events, err := service.GetEvents(rawFilters, timeFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	c.JSON(http.StatusOK, events)
}
