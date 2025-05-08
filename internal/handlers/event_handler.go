package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/wso2/identity-customer-data-service/internal/authentication"
	"github.com/wso2/identity-customer-data-service/internal/errors"
	"github.com/wso2/identity-customer-data-service/internal/models"
	"github.com/wso2/identity-customer-data-service/internal/service"
	"github.com/wso2/identity-customer-data-service/internal/utils"
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

func (s Server) GetEvents(c *gin.Context) {

	rawFilters := c.QueryArray("filter")
	var timeFilter map[string]int
	if timeStr := c.Query("time_range"); timeStr != "" {
		durationSec, err := strconv.Atoi(timeStr) // Parse string to int
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid time_range format, must be an integer representing seconds"})
			return
		}

		currentTime := time.Now().UTC().Unix()        // current time in seconds
		startTime := currentTime - int64(durationSec) // Calculate start time
		timeFilter = map[string]int{
			"event_timestamp_gte": int(startTime), // Use the key expected by Postgres FindEvents
		}
	}

	events, err := service.GetEvents(rawFilters, timeFilter) // Pass the map[string]int
	if err != nil {
		// Consider logging the error server-side as well
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	c.JSON(http.StatusOK, events)
}
