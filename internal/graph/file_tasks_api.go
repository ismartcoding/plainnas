package graph

import (
	"context"
	"sort"

	"ismartcoding/plainnas/internal/graph/model"
)

func createCopyTaskModel(ctx context.Context, ops []*model.FileTaskOpInput) (*model.FileTask, error) {
	converted := make([]fileTaskOp, 0, len(ops))
	for _, op := range ops {
		if op == nil {
			continue
		}
		converted = append(converted, fileTaskOp{Src: op.Src, Dst: op.Dst, Overwrite: op.Overwrite})
	}
	ft, err := createCopyTask(ctx, converted)
	if err != nil {
		return nil, err
	}
	return toModelFileTask(ft), nil
}

func createMoveTaskModel(ctx context.Context, ops []*model.FileTaskOpInput) (*model.FileTask, error) {
	converted := make([]fileTaskOp, 0, len(ops))
	for _, op := range ops {
		if op == nil {
			continue
		}
		converted = append(converted, fileTaskOp{Src: op.Src, Dst: op.Dst, Overwrite: op.Overwrite})
	}
	ft, err := createMoveTask(ctx, converted)
	if err != nil {
		return nil, err
	}
	return toModelFileTask(ft), nil
}

func getTasksModel(ctx context.Context) ([]*model.FileTask, error) {
	clientID, err := getClientIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	dbTasks, err := loadFileTasksFromDB(clientID)
	if err != nil {
		return nil, err
	}

	// Overlay in-memory snapshots for freshest progress.
	mgr := getFileTaskManager()
	mgr.mu.Lock()
	memTasks := make([]*model.FileTask, 0, len(mgr.tasks))
	for _, t := range mgr.tasks {
		if t == nil {
			continue
		}
		t.mu.Lock()
		cid := t.ClientID
		t.mu.Unlock()
		if cid != clientID {
			continue
		}
		memTasks = append(memTasks, toModelFileTask(t))
	}
	mgr.mu.Unlock()

	merged := mergeAndSortTasks(dbTasks, memTasks)
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].UpdatedAt.After(merged[j].UpdatedAt)
	})
	return merged, nil
}
