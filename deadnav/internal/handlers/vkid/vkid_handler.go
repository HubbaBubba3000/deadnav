package vkid

import (
	"deadnav/internal/services"
	"deadnav/internal/services/vkid"
	"net/http"

	"github.com/gin-gonic/gin"
)

// VKIDHandler exposes the VK Mini App auth endpoint.
//
// The Mini App client posts its signed `launch_params` to the API. We verify
// the HMAC-SHA256 signature using our client secret, extract the VK user ID
// and hand the rest off to UserService.LoginWithVK, which returns a JWT
// just like the Telegram auth flow.
type VKIDHandler struct {
	Service     *vkid.VKIDService
	UserService *services.UserService
}

func NewVKIDHandler(service *vkid.VKIDService, userService *services.UserService) *VKIDHandler {
	return &VKIDHandler{Service: service, UserService: userService}
}

// vkAuthRequest is the body posted by the Mini App client.
type vkAuthRequest struct {
	// LaunchParams is the raw `launch_params` query-string the VK client
	// receives when the Mini App starts. The `sign` field is verified
	// server-side against VK_ID_CLIENT_SECRET.
	LaunchParams string `json:"launch_params"`
}

// Login handles a VK Mini App login.
// @Summary Login with VK Mini App launch_params
// @Description Verifies the signed launch_params of a VK Mini App and
// @Description returns a JWT for the corresponding application user.
// @Description Creates a new user on the first login (auth_provider = "vk").
// @Tags auth
// @Accept json
// @Produce json
// @Param request body vkAuthRequest true "VK Mini App auth request"
// @Success 200 {object} services.AuthResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/vk [post]
func (h *VKIDHandler) Login(c *gin.Context) {
	var req vkAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "недопустимый формат запроса"})
		return
	}
	if req.LaunchParams == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "launch_params is required"})
		return
	}

	vkUser, err := h.Service.VerifyLaunchParams(req.LaunchParams)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Build a username from VK first/last name; LoginWithVK falls back to
	// "vk_<id>" if both are empty.
	username := vkUser.FirstName
	if vkUser.LastName != "" {
		if username != "" {
			username += "_"
		}
		username += vkUser.LastName
	}

	authResp, err := h.UserService.LoginWithVK(services.VKAuthRequest{
		VKID:      vkUser.ID,
		Username:  username,
		FirstName: vkUser.FirstName,
		LastName:  vkUser.LastName,
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, authResp)
}
