package db

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
)

type Session struct {
	ClientID   string    `json:"client_id"`
	ClientName string    `json:"client_name"`
	Token      string    `json:"token"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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

func CreateSession(clientID string, clientName string) *Session {
	token := make([]byte, 32)
	rand.Read(token)
	session := &Session{
		ClientID:   clientID,
		ClientName: clientName,
		Token:      base64.StdEncoding.EncodeToString(token),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	data, _ := json.Marshal(session)
	GetDefault().Set([]byte(getSessionKey(clientID)), data, &pebble.WriteOptions{Sync: true})
	return session
}

func UpdateSession(clientID string, clientName string) *Session {
	var session Session
	err := GetDefault().LoadJSON(getSessionKey(clientID), &session)
	if err != nil {
		return nil
	}

	token := make([]byte, 32)
	rand.Read(token)
	session.ClientName = clientName
	session.Token = base64.StdEncoding.EncodeToString(token)
	session.UpdatedAt = time.Now()

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
