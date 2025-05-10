package handler

import (
	"encoding/json"
	"github.com/wso2/identity-customer-data-service/internal/profile/model"
	"github.com/wso2/identity-customer-data-service/internal/profile/service"
	"github.com/wso2/identity-customer-data-service/internal/system/constants"
	"github.com/wso2/identity-customer-data-service/internal/system/logger"
	"github.com/wso2/identity-customer-data-service/internal/utils"
	"net/http"
	"strings"
	"sync"
)

type ProfileHandler struct {
	store map[string]model.Profile
	mu    *sync.RWMutex
}

func NewProfileHandler() *ProfileHandler {

	return &ProfileHandler{
		store: make(map[string]model.Profile),
		mu:    &sync.RWMutex{},
	}
}

// GetProfile handles profile retrieval requests
func (ph *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	profileId := pathParts[len(pathParts)-1]
	var profile *model.Profile
	var err error
	profile, err = service.GetProfile(profileId)
	if err != nil {
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profile)
}

// DeleteProfile handles profile deletion
func (ph *ProfileHandler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	profileId := pathParts[len(pathParts)-1]
	err := service.DeleteProfile(profileId)
	if err != nil {
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetAllProfiles handles profile retrieval with and without filters
func (ph *ProfileHandler) GetAllProfiles(w http.ResponseWriter, r *http.Request) {

	logger.Info("Fetching all profiles without filters")
	var profiles []model.Profile
	var err error
	// Build the filter from query params
	filter := r.URL.Query()[constants.Filter] // Handles multiple filters
	if len(filter) > 0 {
		profiles, err = service.GetAllProfilesWithFilter(filter)
	} else {
		logger.Info("Fetching all profiles without filters")
		profiles, err = service.GetAllProfiles()
	}
	if err != nil {
		utils.HandleHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(profiles)
}
