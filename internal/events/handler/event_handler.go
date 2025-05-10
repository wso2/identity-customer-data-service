package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/events/service"
	"github.com/wso2/identity-customer-data-service/internal/system/authentication"

	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
	"github.com/wso2/identity-customer-data-service/internal/utils"
)

type EventHandler struct {
	store map[string]model.Event
	mu    *sync.RWMutex
}

func NewEventHandler() *EventHandler {

	return &EventHandler{
		store: make(map[string]model.Event),
		mu:    &sync.RWMutex{},
	}
}

// AddEvent handles adding a single event
func (eh *EventHandler) AddEvent(w http.ResponseWriter, r *http.Request) {
	if _, err := authentication.ValidateAuthentication(r); err != nil {
		http.Error(w, "Unauthorized request", http.StatusUnauthorized)
		return
	}

	var event model.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	queue := &workers.ProfileWorkerQueue{}
	if err := service.AddEvents(event, queue); err != nil {
		utils.HandleHTTPError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// GetEvent fetches a specific event
func (eh *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	eventId := r.URL.Query().Get("event_id")
	if eventId == "" {
		http.Error(w, "Missing event_id parameter", http.StatusBadRequest)
		return
	}

	events, err := service.GetEvent(eventId)
	if err != nil {
		http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(events)
}

// GetEvents fetches all events with optional filters
func (eh *EventHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	rawFilters := r.URL.Query()["filter"]
	var timeFilter map[string]int
	if timeStr := r.URL.Query().Get("time_range"); timeStr != "" {
		durationSec, err := strconv.Atoi(timeStr)
		if err != nil {
			http.Error(w, "Invalid time_range format, must be an integer representing seconds", http.StatusBadRequest)
			return
		}

		currentTime := time.Now().UTC().Unix()
		startTime := currentTime - int64(durationSec)
		timeFilter = map[string]int{
			"event_timestamp_gte": int(startTime),
		}
	}

	events, err := service.GetEvents(rawFilters, timeFilter)
	if err != nil {
		http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(events)
}

func (eh *EventHandler) GetWriteKey(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	applicationId := pathParts[len(pathParts)-1]
	// Step 1: Get existing token if needed (for now assume no previous token available)
	// If you have a DB or cache, fetch the existing token here.
	existingToken, _ := authentication.GetTokenFromIS(applicationId)

	// Step 2: If token exists, revoke it first as this would be re-generating a new one
	if existingToken != "" {
		err := authentication.RevokeToken(existingToken)
		if err != nil {
			utils.HandleHTTPError(w, err)
			return
		}
	}

	// Step 3: Get a new token
	newToken, err := authentication.GetTokenFromIS(applicationId)
	if err != nil {
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"write_key": newToken,
	})
}
