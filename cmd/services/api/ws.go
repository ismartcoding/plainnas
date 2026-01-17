package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/pkg/eventbus"
	"ismartcoding/plainnas/internal/strutils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	wsSessionsMu sync.Mutex
	wsSessions   = map[string]*websocket.Conn{}
)

func wsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		cid := c.Query("cid")
		if cid == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		session := db.GetSession(cid)
		if session == nil {
			conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "invalid_request"), time.Now().Add(time.Second))
			conn.Close()
			return
		}

		key, _ := base64.StdEncoding.DecodeString(session.Token)

		_, msg, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			return
		}
		decrypted := strutils.ChaCha20Decrypt(key, msg)
		if decrypted == nil {
			conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "invalid_request"), time.Now().Add(time.Second))
			conn.Close()
			return
		}

		wsSessionsMu.Lock()
		wsSessions[cid] = conn
		wsSessionsMu.Unlock()

		go func(id string, cconn *websocket.Conn, key []byte) {
			defer func() {
				// best-effort unsubscribe happens below
				wsSessionsMu.Lock()
				delete(wsSessions, id)
				wsSessionsMu.Unlock()
				cconn.Close()
			}()
			scanHandler := func(payload map[string]any) {
				b, _ := json.Marshal(payload)
				if enc := strutils.ChaCha20Encrypt(key, b); enc != nil {
					_ = cconn.WriteMessage(websocket.BinaryMessage, append(int32ToBytes(4), enc...))
				}
			}
			_ = eventbus.GetDefault().Subscribe(consts.EVENT_MEDIA_SCAN_PROGRESS, scanHandler)
			defer func() { _ = eventbus.GetDefault().Unsubscribe(consts.EVENT_MEDIA_SCAN_PROGRESS, scanHandler) }()

			fileTaskHandler := func(eventCID string, payload map[string]any) {
				if eventCID != id {
					return
				}
				b, _ := json.Marshal(payload)
				if enc := strutils.ChaCha20Encrypt(key, b); enc != nil {
					_ = cconn.WriteMessage(websocket.BinaryMessage, append(int32ToBytes(6), enc...))
				}
			}
			_ = eventbus.GetDefault().Subscribe(consts.EVENT_FILE_TASK_PROGRESS, fileTaskHandler)
			defer func() { _ = eventbus.GetDefault().Unsubscribe(consts.EVENT_FILE_TASK_PROGRESS, fileTaskHandler) }()
			for {
				if _, _, err := cconn.ReadMessage(); err != nil {
					return
				}
			}
		}(cid, conn, key)
	}
}

func int32ToBytes(v int32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}
