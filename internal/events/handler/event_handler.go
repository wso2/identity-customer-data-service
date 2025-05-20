package handler

import (
	"encoding/json"
	"github.com/wso2/identity-customer-data-service/internal/events/provider"
	"github.com/wso2/identity-customer-data-service/internal/system/utils"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/wso2/identity-customer-data-service/internal/system/authentication"

	"github.com/wso2/identity-customer-data-service/internal/events/model"
	"github.com/wso2/identity-customer-data-service/internal/system/workers"
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

	var event model.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// todo: ideally this has to be the first step. For that, even before extracting the
	// payload should be able to get the app/orgid from path. Need a modification
	if _, err := authentication.ValidateAuthenticationForEvent(r, event); err != nil {
		http.Error(w, "Unauthorized request", http.StatusUnauthorized)
		return
	}

	queue := &workers.ProfileWorkerQueue{}
	eventsProvider := provider.NewEventsProvider()
	eventsService := eventsProvider.GetEventsService()
	if err := eventsService.AddEvents(event, queue); err != nil {
		utils.HandleError(w, err)
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

	eventsProvider := provider.NewEventsProvider()
	eventsService := eventsProvider.GetEventsService()
	events, err := eventsService.GetEvent(eventId)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(events)
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

	eventsProvider := provider.NewEventsProvider()
	eventsService := eventsProvider.GetEventsService()
	events, err := eventsService.GetEvents(rawFilters, timeFilter)
	if err != nil {
		utils.HandleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(events)
}
