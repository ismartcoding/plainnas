package db

import (
	"strings"
	"time"

	"ismartcoding/plainnas/internal/pkg/shortid"
)

const eventsKey = "events"

// Keep events bounded to avoid unbounded DB growth.
const maxEvents = 1000

type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	ClientID  string    `json:"client_id"`
	CreatedAt time.Time `json:"created_at"`
}

func AddEvent(eventType string, message string, clientID string) {
	eventType = strings.TrimSpace(eventType)
	message = strings.TrimSpace(message)
	clientID = strings.TrimSpace(clientID)
	if eventType == "" || message == "" {
		return
	}

	e := Event{
		ID:        shortid.New(),
		Type:      eventType,
		Message:   message,
		ClientID:  clientID,
		CreatedAt: time.Now().UTC(),
	}

	var stored []Event
	_ = GetDefault().LoadJSON(eventsKey, &stored)
	stored = append([]Event{e}, stored...)
	if len(stored) > maxEvents {
		stored = stored[:maxEvents]
	}
	_ = GetDefault().StoreJSON(eventsKey, stored)
}

func GetEvents(limit int) []Event {
	var stored []Event
	_ = GetDefault().LoadJSON(eventsKey, &stored)
	if limit > 0 && len(stored) > limit {
		return stored[:limit]
	}
	return stored
}
