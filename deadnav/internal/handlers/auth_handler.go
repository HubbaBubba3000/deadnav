package handlers

import (
	"deadnav/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	UserService *services.UserService
}

func NewAuthHandler(userService *services.UserService) *AuthHandler {
	return &AuthHandler{
		UserService: userService,
	}
}

// Register handles user registration
// @Summary Register a new user
// @Description Register a new user with username, email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body services.RegisterRequest true "Register request"
// @Success 200 {object} services.AuthResponse
// @Failure 400 {object} map[string]string
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req services.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.UserService.Register(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Login handles user login with username/email and password
// @Summary Login user
// @Description Login with username/email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body services.LoginRequest true "Login request"
// @Success 200 {object} services.AuthResponse
// @Failure 400 {object} map[string]string
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.UserService.Login(req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// LoginWithTelegram handles Telegram authentication
// @Summary Login with Telegram
// @Description Login or register a user via Telegram
// @Tags auth
// @Accept json
// @Produce json
// @Param request body services.TelegramAuthRequest true "Telegram auth request"
// @Success 200 {object} services.AuthResponse
// @Failure 400 {object} map[string]string
// @Router /api/v1/auth/telegram [post]
func (h *AuthHandler) LoginWithTelegram(c *gin.Context) {
	var req services.TelegramAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.UserService.LoginWithTelegram(req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetMe returns the current authenticated user
// @Summary Get current user
// @Description Get the current authenticated user info
// @Tags auth
// @Produce json
// @Success 200 {object} models.User
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.UserService.GetUserByID(userID.(int64))
	if err != nil {
		internalError(c, "GetMe: fetch", err)
		return
	}

	c.JSON(http.StatusOK, user)
}
