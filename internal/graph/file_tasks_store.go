package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
)

const fileTaskDBPrefix = "filetask:"

func fileTaskKey(clientID string, taskID string) string {
	return fileTaskDBPrefix + clientID + ":" + taskID
}

func saveFileTaskToDB(clientID string, task *model.FileTask) error {
	if clientID == "" || task == nil || task.ID == "" {
		return nil
	}
	return db.GetDefault().StoreJSON(fileTaskKey(clientID, task.ID), task)
}

func loadFileTasksFromDB(clientID string) ([]*model.FileTask, error) {
	if clientID == "" {
		return nil, fmt.Errorf("unauthorized")
	}

	prefix := []byte(fileTaskDBPrefix + clientID + ":")
	var tasks []*model.FileTask

	err := db.GetDefault().Iterate(prefix, func(_ []byte, value []byte) error {
		var t model.FileTask
		if err := json.Unmarshal(value, &t); err != nil {
			return nil
		}
		copy := t
		tasks = append(tasks, &copy)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
	})
	return tasks, nil
}

func getClientIDFromContext(ctx context.Context) (string, error) {
	clientID, _ := ctx.Value(ContextKeyClientID).(string)
	if clientID == "" {
		return "", fmt.Errorf("unauthorized")
	}
	return clientID, nil
}

func mergeAndSortTasks(dbTasks []*model.FileTask, memTasks []*model.FileTask) []*model.FileTask {
	byID := map[string]*model.FileTask{}
	for _, t := range dbTasks {
		if t != nil {
			byID[t.ID] = t
		}
	}
	for _, t := range memTasks {
		if t != nil {
			byID[t.ID] = t
		}
	}

	out := make([]*model.FileTask, 0, len(byID))
	for _, t := range byID {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// persistThrottle helps avoid writing to Pebble too frequently.
// Writes at most once per second unless forced.
func persistThrottle(now time.Time, last *time.Time, force bool) bool {
	if force {
		*last = now
		return true
	}
	if last == nil || last.IsZero() {
		if last != nil {
			*last = now
		}
		return true
	}
	if now.Sub(*last) >= time.Second {
		*last = now
		return true
	}
	return false
}
