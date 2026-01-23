package db

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
)

type Session struct {
	ClientID       string    `json:"client_id"`
	ClientName     string    `json:"client_name"`
	BrowserName    string    `json:"browser_name"`
	BrowserVersion string    `json:"browser_version"`
	OSName         string    `json:"os_name"`
	OSVersion      string    `json:"os_version"`
	IsMobile       bool      `json:"is_mobile"`
	Token          string    `json:"token"`
	LastActive     time.Time `json:"last_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type SessionClientInfo struct {
	ClientName     string
	BrowserName    string
	BrowserVersion string
	OSName         string
	OSVersion      string
	IsMobile       bool
}

func getSessionKey(clientID string) string {
	return fmt.Sprintf("session:%s", clientID)
}

func GetSession(clientID string) *Session {
	var session Session
	err := GetDefault().LoadJSON(getSessionKey(clientID), &session)
	if err != nil || session.ClientID == "" {
		return nil
	}
	return &session
}

func CreateSession(clientID string, info SessionClientInfo) *Session {
	token := make([]byte, 32)
	rand.Read(token)
	now := time.Now().UTC()
	session := &Session{
		ClientID:       clientID,
		ClientName:     info.ClientName,
		BrowserName:    info.BrowserName,
		BrowserVersion: info.BrowserVersion,
		OSName:         info.OSName,
		OSVersion:      info.OSVersion,
		IsMobile:       info.IsMobile,
		Token:          base64.StdEncoding.EncodeToString(token),
		LastActive:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	data, _ := json.Marshal(session)
	GetDefault().Set([]byte(getSessionKey(clientID)), data, &pebble.WriteOptions{Sync: true})
	return session
}

func TouchSessionLastActive(session *Session) bool {
	if session == nil || strings.TrimSpace(session.ClientID) == "" {
		return false
	}

	session.LastActive = time.Now().UTC()
	data, err := json.Marshal(session)
	if err != nil {
		return false
	}

	// Avoid fsync on every request; lastActive is best-effort.
	GetDefault().Set([]byte(getSessionKey(session.ClientID)), data, &pebble.WriteOptions{Sync: false})
	return true
}

func UpdateSession(clientID string, info SessionClientInfo) *Session {
	var session Session
	err := GetDefault().LoadJSON(getSessionKey(clientID), &session)
	if err != nil {
		return nil
	}

	token := make([]byte, 32)
	rand.Read(token)
	session.ClientName = info.ClientName
	session.BrowserName = info.BrowserName
	session.BrowserVersion = info.BrowserVersion
	session.OSName = info.OSName
	session.OSVersion = info.OSVersion
	session.IsMobile = info.IsMobile
	session.Token = base64.StdEncoding.EncodeToString(token)
	session.UpdatedAt = time.Now().UTC()

	data, _ := json.Marshal(session)
	GetDefault().Set([]byte(getSessionKey(clientID)), data, &pebble.WriteOptions{Sync: true})
	return &session
}

func GetAllSessions() []Session {
	var sessions []Session
	GetDefault().Iterate([]byte("session:"), func(key []byte, value []byte) error {
		var session Session
		if err := json.Unmarshal(value, &session); err != nil {
			return err
		}
		sessions = append(sessions, session)
		return nil
	})
	return sessions
}

func RevokeSession(clientID string) bool {
	if strings.TrimSpace(clientID) == "" {
		return false
	}
	if err := GetDefault().Delete([]byte(getSessionKey(clientID))); err != nil {
		return false
	}
	return true
}
