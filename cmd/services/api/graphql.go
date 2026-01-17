package api

import (
	"bytes"
	"context"
	"io"
	"ismartcoding/plainnas/internal/config"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph"
	"ismartcoding/plainnas/internal/graph/generated"
	"ismartcoding/plainnas/internal/pkg/log"
	"ismartcoding/plainnas/internal/strutils"
	"net/http"
	"strings"

	"encoding/base64"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gin-gonic/gin"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func createErrorResponse(msg string) *graphql.Response {
	err := &gqlerror.Error{Message: msg}
	return &graphql.Response{Errors: []*gqlerror.Error{err}}
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID := c.GetHeader("c-id")
		if clientID != "" {
			session := db.GetSession(clientID)
			if session == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, createErrorResponse("Unauthorized"))
			} else {
				var rawBody []byte
				if c.Request.Body != nil {
					rawBody, _ = io.ReadAll(c.Request.Body)
				}

				key, _ := base64.StdEncoding.DecodeString(session.Token)
				decryptedBody := strutils.ChaCha20Decrypt(key, rawBody)
				if decryptedBody == nil {
					log.Errorf("Failed to decrypt request body")
					c.AbortWithStatusJSON(http.StatusBadRequest, createErrorResponse("Decryption failed"))
					return
				}
				log.Debugf("Request body: %s", string(decryptedBody))
				c.Request.Body = io.NopCloser(bytes.NewBuffer(decryptedBody))
				blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
				c.Writer = blw
				c.Next()
				encryptedResponse := strutils.ChaCha20Encrypt(key, blw.body.Bytes())
				if encryptedResponse == nil {
					log.Errorf("Failed to encrypt response body")
					c.AbortWithStatusJSON(http.StatusInternalServerError, createErrorResponse("Encryption failed"))
					return
				}
				blw.ResponseWriter.Write(encryptedResponse)
			}
		} else {
			// for dev
			authorization := c.GetHeader("authorization")
			if authorization != "" {
				pairs := strings.Split(authorization, " ")
				token := ""
				if len(pairs) > 1 {
					token = pairs[1]
				}
				devToken := config.GetDefault().GetString("auth.dev_token")
				if token == "" || token != devToken {
					c.AbortWithStatusJSON(http.StatusUnauthorized, createErrorResponse("Unauthorized: token is invalid"))
				} else {
					c.Next()
				}
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, createErrorResponse("Unauthorized: make sure add http headers `{\"authorization\": \"Bearer <dev_token>\"}`"))
			}
		}
	}
}

func graphqlHandler() gin.HandlerFunc {
	h := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{}}))
	h.AddTransport(transport.POST{})
	h.Use(extension.Introspection{})
	h.SetErrorPresenter(func(ctx context.Context, e error) *gqlerror.Error {
		err := graphql.DefaultErrorPresenter(ctx, e)
		log.Error(err)
		return err
	})

	return func(c *gin.Context) {
		clientID := c.GetHeader("c-id")
		req := c.Request.Clone(context.WithValue(c.Request.Context(), graph.ContextKeyClientID, clientID))
		// Force JSON so gqlgen doesn't try to parse multipart when frontend sets multipart/form-data
		req.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(c.Writer, req)
	}
}
