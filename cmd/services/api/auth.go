package api

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"ismartcoding/plainnas/internal/config"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/strutils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type authRequestData struct {
	Password string `json:"password"`
}

type authResponseData struct {
	NasID string `json:"nas_id"`
	Token string `json:"token"`
}

func getHashPwd() string {
	pwd := config.GetDefault().GetString("auth.password")
	s := sha512.New()
	s.Write([]byte(pwd))
	return hex.EncodeToString(s.Sum(nil))
}

func authHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var data authRequestData
		if err := c.ShouldBindJSON(&data); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Bad request"))
			return
		}

		token := ""
		if getHashPwd() == data.Password {
			clientID := c.GetHeader("c-id")
			if clientID == "" {
				c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Missing client id"))
				return
			}
			session := db.GetSession(clientID)
			if session == nil {
				session = db.CreateSession(clientID, "")
			} else {
				session = db.UpdateSession(clientID, "")
			}
			if session != nil {
				token = session.Token
			}
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, createErrorResponse("Unauthorized"))
			return
		}

		// Encrypt response using first 32 bytes of provided password hash
		resp := authResponseData{
			NasID: config.GetDefault().GetString("nas.id"),
			Token: token,
		}
		b, _ := json.Marshal(resp)
		key := []byte(data.Password)
		if len(key) < 32 {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Bad request"))
			return
		}
		encrypted := strutils.ChaCha20Encrypt(key[:32], b)
		if encrypted == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, createErrorResponse("Encryption failed"))
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", encrypted)
	}
}
