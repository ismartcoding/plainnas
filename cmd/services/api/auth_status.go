package api

import (
	"encoding/base64"
	"io"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/strutils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func authStatusHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID := c.GetHeader("c-id")
		if clientID == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("`c-id` is missing in the headers"))
			return
		}

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
					c.Status(http.StatusOK)
					return
				}
			}
		}

		c.Status(http.StatusNoContent)
	}
}
