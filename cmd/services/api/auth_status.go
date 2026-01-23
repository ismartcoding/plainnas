package api

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/strutils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type authStatusResponse struct {
	Authenticated bool `json:"authenticated"`
	NeedsSetup    bool `json:"needsSetup"`
}

func authStatusHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID := c.GetHeader("c-id")
		if clientID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("`c-id` is missing in the headers"))
			return
		}

		needsSetup := !db.HasAdminPassword()
		authed := false

		var rawBody []byte
		if c.Request.Body != nil {
			rawBody, _ = io.ReadAll(c.Request.Body)
		}

		if len(rawBody) > 0 {
			session := db.GetSession(clientID)
			if session != nil {
				key, _ := base64.StdEncoding.DecodeString(session.Token)
				decrypted := strutils.ChaCha20Decrypt(key, rawBody)
				if decrypted != nil {
					// Token is valid.
					authed = true
				}
			}
		}

		// If initial password is not configured, force unauthenticated.
		if needsSetup {
			authed = false
		}

		resp := authStatusResponse{Authenticated: authed, NeedsSetup: needsSetup}
		b, _ := json.Marshal(resp)
		c.Data(http.StatusOK, "application/json", b)
	}
}
