package graph

import (
	"context"
	"sort"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

func listSessions(_ context.Context) ([]*model.Session, error) {
	sessions := db.GetAllSessions()
	out := make([]*model.Session, 0, len(sessions))
	for i := range sessions {
		s := sessions[i]
		lastActive := s.LastActive
		if lastActive.IsZero() {
			lastActive = s.UpdatedAt
		}
		out = append(out, &model.Session{
			ClientID:   s.ClientID,
			ClientName: s.ClientName,
			LastActive: lastActive,
			CreatedAt:  s.CreatedAt,
			UpdatedAt:  s.UpdatedAt,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})

	return out, nil
}

func revokeSession(_ context.Context, clientID string) (bool, error) {
	if strings.TrimSpace(clientID) == "" {
		return false, nil
	}
	if s := db.GetSession(clientID); s != nil {
		name := strings.TrimSpace(s.ClientName)
		db.AddEvent("revoke", name, clientID)
	} else {
		db.AddEvent("revoke", "", clientID)
	}
	return db.RevokeSession(clientID), nil
}

func logout(ctx context.Context) (bool, error) {
	clientID, _ := ctx.Value(ContextKeyClientID).(string)
	if strings.TrimSpace(clientID) == "" {
		return false, nil
	}
	if s := db.GetSession(clientID); s != nil {
		name := strings.TrimSpace(s.ClientName)
		db.AddEvent("logout", name, clientID)
	} else {
		db.AddEvent("logout", "", clientID)
	}
	return db.RevokeSession(clientID), nil
}
