package api

import (
	"encoding/json"
	"io"
	"ismartcoding/plainnas/internal/db"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type authSetupRequest struct {
	Password string `json:"password"` // SHA-512(hex) string produced by web
}

func isHexLowerOrUpper(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		isDigit := c >= '0' && c <= '9'
		isLower := c >= 'a' && c <= 'f'
		isUpper := c >= 'A' && c <= 'F'
		if !isDigit && !isLower && !isUpper {
			return false
		}
	}
	return true
}

func authSetupHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if db.HasAdminPassword() {
			c.AbortWithStatusJSON(http.StatusConflict, createErrorResponse("Password already configured"))
			return
		}

		rawBody, _ := io.ReadAll(c.Request.Body)
		if len(rawBody) == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Bad request"))
			return
		}

		var req authSetupRequest
		if err := json.Unmarshal(rawBody, &req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Bad request"))
			return
		}
		h := strings.TrimSpace(req.Password)
		if len(h) != 128 || !isHexLowerOrUpper(h) {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Invalid password hash"))
			return
		}

		if err := db.SetAdminPasswordHash(h); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, createErrorResponse("Failed to save password"))
			return
		}

		c.Status(http.StatusOK)
	}
}
