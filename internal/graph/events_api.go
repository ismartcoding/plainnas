package graph

import (
	"context"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

func listEvents(_ context.Context, limit int) ([]*model.Event, error) {
	events := db.GetEvents(limit)
	out := make([]*model.Event, 0, len(events))
	for i := range events {
		e := events[i]
		out = append(out, &model.Event{
			ID:        e.ID,
			Type:      e.Type,
			Message:   e.Message,
			ClientID:  e.ClientID,
			CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}
