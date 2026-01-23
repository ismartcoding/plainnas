package api

import (
	"encoding/json"
	"io"
	"ismartcoding/plainnas/internal/config"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/strutils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type authRequestData struct {
	Password       string `json:"password"`
	BrowserName    string `json:"browserName"`
	BrowserVersion string `json:"browserVersion"`
	OSName         string `json:"osName"`
	OSVersion      string `json:"osVersion"`
	IsMobile       bool   `json:"isMobile"`
}

type authResponseData struct {
	NasID string `json:"nas_id"`
	Token string `json:"token"`
}

func getHashPwd() string {
	// Password is stored in DB as a SHA-512(hex) string.
	return db.GetAdminPasswordHash()
}

func authHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID := c.GetHeader("c-id")

		if !db.HasAdminPassword() {
			c.AbortWithStatusJSON(http.StatusConflict, createErrorResponse("Password not configured"))
			return
		}

		var rawBody []byte
		if c.Request.Body != nil {
			rawBody, _ = io.ReadAll(c.Request.Body)
		}
		if len(rawBody) == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Bad request"))
			return
		}

		hashPwd := getHashPwd()
		key := []byte(hashPwd)
		if len(key) < 32 {
			c.AbortWithStatusJSON(http.StatusInternalServerError, createErrorResponse("Server misconfigured"))
			return
		}
		decrypted := strutils.ChaCha20Decrypt(key[:32], rawBody)
		if decrypted == nil {
			db.AddEvent("login_failed", "decrypt_failed", clientID)
			c.AbortWithStatusJSON(http.StatusUnauthorized, createErrorResponse("Unauthorized"))
			return
		}

		var data authRequestData
		if err := json.Unmarshal(decrypted, &data); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Bad request"))
			return
		}

		token := ""
		clientName := ""
		if hashPwd == data.Password {
			if clientID == "" {
				db.AddEvent("login_failed", "missing_client_id", "")
				c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Missing client id"))
				return
			}

			if data.BrowserName != "" {
				clientName = data.BrowserName
				if data.BrowserVersion != "" {
					clientName += " " + data.BrowserVersion
				}
				if data.OSName != "" {
					clientName += " / " + data.OSName
					if data.OSVersion != "" {
						clientName += " " + data.OSVersion
					}
				}
				if data.IsMobile {
					clientName += " (Mobile)"
				}
			}

			info := db.SessionClientInfo{
				ClientName:     clientName,
				BrowserName:    data.BrowserName,
				BrowserVersion: data.BrowserVersion,
				OSName:         data.OSName,
				OSVersion:      data.OSVersion,
				IsMobile:       data.IsMobile,
			}
			session := db.GetSession(clientID)
			if session == nil {
				session = db.CreateSession(clientID, info)
			} else {
				session = db.UpdateSession(clientID, info)
			}
			if session != nil {
				token = session.Token
				db.AddEvent("login", clientName, clientID)
			}
		} else {
			db.AddEvent("login_failed", "bad_password", clientID)
			c.AbortWithStatusJSON(http.StatusUnauthorized, createErrorResponse("Unauthorized"))
			return
		}

		// Encrypt response using first 32 bytes of provided password hash
		resp := authResponseData{
			NasID: config.GetDefault().GetString("nas.id"),
			Token: token,
		}
		b, _ := json.Marshal(resp)
		respKey := []byte(data.Password)
		if len(respKey) < 32 {
			c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Bad request"))
			return
		}
		encrypted := strutils.ChaCha20Encrypt(respKey[:32], b)
		if encrypted == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, createErrorResponse("Encryption failed"))
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", encrypted)
	}
}
