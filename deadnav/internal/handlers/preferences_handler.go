package handlers

import (
	"net/http"

	"deadnav/internal/models"
	"deadnav/internal/services"

	"github.com/gin-gonic/gin"
)

// PreferencesHandler handles HTTP requests for user scheduling preferences.
type PreferencesHandler struct {
	prefsService *services.PreferencesService
}

// NewPreferencesHandler creates a PreferencesHandler with the given service.
func NewPreferencesHandler(prefsService *services.PreferencesService) *PreferencesHandler {
	return &PreferencesHandler{prefsService: prefsService}
}

// GetPreferences godoc
// @Summary Get scheduling preferences
// @Description Return the authenticated user's scheduling preferences.
// If no preferences have been saved yet, the defaults are returned.
// @Tags preferences
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.UserPreferences
// @Failure 401 {object} errorResponse
// @Router /api/v1/preferences [get]
func (h *PreferencesHandler) GetPreferences(c *gin.Context) {
	userID := mustUserID(c)

	prefs, err := h.prefsService.GetPreferences(userID)
	if err != nil {
		internalError(c, "GetPreferences: fetch", err)
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// UpdatePreferences godoc
// @Summary Update scheduling preferences
// @Description Create or replace the authenticated user's scheduling preferences.
// @Tags preferences
// @Accept  json
// @Produce json
// @Security BearerAuth
// @Param   preferences body models.UserPreferences true "Preferences payload"
// @Success 200 {object} models.UserPreferences
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /api/v1/preferences [put]
func (h *PreferencesHandler) UpdatePreferences(c *gin.Context) {
	userID := mustUserID(c)

	var prefs models.UserPreferences
	if err := c.ShouldBindJSON(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	prefs.UserID = userID

	if err := h.prefsService.UpsertPreferences(&prefs); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// Return the saved preferences.
	saved, err := h.prefsService.GetPreferences(userID)
	if err != nil {
		internalError(c, "UpdatePreferences: re-fetch", err)
		return
	}

	c.JSON(http.StatusOK, saved)
}
