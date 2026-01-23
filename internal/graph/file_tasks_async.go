package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/pkg/eventbus"
	"ismartcoding/plainnas/internal/pkg/shortid"
)

type fileTaskType string

type fileTaskStatus string

const (
	fileTaskTypeCopy fileTaskType = "COPY"
	fileTaskTypeMove fileTaskType = "MOVE"

	fileTaskStatusQueued  fileTaskStatus = "QUEUED"
	fileTaskStatusRunning fileTaskStatus = "RUNNING"
	fileTaskStatusDone    fileTaskStatus = "DONE"
	fileTaskStatusError   fileTaskStatus = "ERROR"
)

type fileTaskOp struct {
	Src       string
	Dst       string
	Overwrite bool
}

type fileTask struct {
	mu sync.Mutex

	ID          string
	ClientID    string
	Type        fileTaskType
	Title       string
	Status      fileTaskStatus
	Error       string
	TotalBytes  int64
	DoneBytes   int64
	TotalItems  int64
	DoneItems   int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	lastPersist time.Time

	Ops []fileTaskOp
}

type fileTaskManager struct {
	mu    sync.Mutex
	tasks map[string]*fileTask
	queue chan string
}

var (
	fileTasksOnce sync.Once
	fileTasksMgr  *fileTaskManager
)

func getFileTaskManager() *fileTaskManager {
	fileTasksOnce.Do(func() {
		m := &fileTaskManager{
			tasks: map[string]*fileTask{},
			queue: make(chan string, 256),
		}
		fileTasksMgr = m
		go m.worker()
	})
	return fileTasksMgr
}

func (m *fileTaskManager) create(clientID string, typ fileTaskType, title string, ops []fileTaskOp) *fileTask {
	now := time.Now().UTC()
	t := &fileTask{
		ID:        shortid.New(),
		ClientID:  clientID,
		Type:      typ,
		Title:     title,
		Status:    fileTaskStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
		Ops:       ops,
	}

	m.mu.Lock()
	m.tasks[t.ID] = t
	m.mu.Unlock()

	m.publishSnapshot(t)
	m.queue <- t.ID
	return t
}

func (m *fileTaskManager) get(id string) *fileTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks[id]
}

func (m *fileTaskManager) publishSnapshot(t *fileTask) {
	t.mu.Lock()
	cid := t.ClientID
	snap := map[string]any{
		"id":         t.ID,
		"type":       string(t.Type),
		"title":      t.Title,
		"status":     string(t.Status),
		"error":      t.Error,
		"totalBytes": t.TotalBytes,
		"doneBytes":  t.DoneBytes,
		"totalItems": t.TotalItems,
		"doneItems":  t.DoneItems,
		"createdAt":  t.CreatedAt,
		"updatedAt":  t.UpdatedAt,
	}
	modelTask := &model.FileTask{
		ID:         t.ID,
		Type:       model.FileTaskType(t.Type),
		Title:      t.Title,
		Status:     model.FileTaskStatus(t.Status),
		Error:      t.Error,
		TotalBytes: int(t.TotalBytes),
		DoneBytes:  int(t.DoneBytes),
		TotalItems: int(t.TotalItems),
		DoneItems:  int(t.DoneItems),
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
	forcePersist := t.Status == fileTaskStatusDone || t.Status == fileTaskStatusError
	now := time.Now().UTC()
	shouldPersist := persistThrottle(now, &t.lastPersist, forcePersist)
	t.mu.Unlock()

	eventbus.GetDefault().Publish(consts.EVENT_FILE_TASK_PROGRESS, cid, snap)
	if shouldPersist {
		_ = saveFileTaskToDB(cid, modelTask)
	}
}

func (m *fileTaskManager) worker() {
	for id := range m.queue {
		t := m.get(id)
		if t == nil {
			continue
		}
		m.run(t)
	}
}

func (m *fileTaskManager) run(t *fileTask) {
	t.mu.Lock()
	t.Status = fileTaskStatusRunning
	t.Error = ""
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	m.publishSnapshot(t)

	// Precompute totals for stable progress percentage.
	var totalBytes int64
	var totalItems int64
	for _, op := range t.Ops {
		b, i, err := computeTotals(op.Src)
		if err != nil {
			m.fail(t, err)
			return
		}
		totalBytes += b
		totalItems += i
	}

	t.mu.Lock()
	t.TotalBytes = totalBytes
	t.TotalItems = totalItems
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	m.publishSnapshot(t)

	var (
		lastEmit = time.Now().UTC()
		emitMu   sync.Mutex
	)

	emit := func(force bool) {
		emitMu.Lock()
		defer emitMu.Unlock()
		if force || time.Since(lastEmit) >= 200*time.Millisecond {
			lastEmit = time.Now().UTC()
			m.publishSnapshot(t)
		}
	}

	progress := &fileOpProgress{
		AddBytes: func(n int64) {
			t.mu.Lock()
			t.DoneBytes += n
			t.UpdatedAt = time.Now().UTC()
			t.mu.Unlock()
			emit(false)
		},
		AddItem: func() {
			t.mu.Lock()
			t.DoneItems++
			t.UpdatedAt = time.Now().UTC()
			t.mu.Unlock()
			emit(false)
		},
	}

	for _, op := range t.Ops {
		var err error
		switch t.Type {
		case fileTaskTypeCopy:
			_, err = copyFileOpWithProgress(op.Src, op.Dst, op.Overwrite, progress)
		case fileTaskTypeMove:
			_, err = moveFileOpWithProgress(op.Src, op.Dst, op.Overwrite, progress)
		default:
			err = fmt.Errorf("unknown task type")
		}
		if err != nil {
			m.fail(t, err)
			return
		}
		emit(true)
	}

	t.mu.Lock()
	t.Status = fileTaskStatusDone
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	m.publishSnapshot(t)
}

func (m *fileTaskManager) fail(t *fileTask, err error) {
	t.mu.Lock()
	t.Status = fileTaskStatusError
	t.Error = err.Error()
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	m.publishSnapshot(t)
}

func computeTotals(src string) (bytes int64, items int64, err error) {
	fi, err := os.Stat(filepath.Clean(src))
	if err != nil {
		return 0, 0, err
	}
	if !fi.IsDir() {
		return fi.Size(), 1, nil
	}

	var totalBytes int64
	var totalItems int64
	walkErr := filepath.WalkDir(src, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		totalBytes += info.Size()
		totalItems++
		return nil
	})
	if walkErr != nil {
		return 0, 0, walkErr
	}
	return totalBytes, totalItems, nil
}

func createCopyTask(ctx context.Context, ops []fileTaskOp) (*fileTask, error) {
	clientID, _ := ctx.Value(ContextKeyClientID).(string)
	if clientID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operations")
	}
	return getFileTaskManager().create(clientID, fileTaskTypeCopy, "Copy files", ops), nil
}

func createMoveTask(ctx context.Context, ops []fileTaskOp) (*fileTask, error) {
	clientID, _ := ctx.Value(ContextKeyClientID).(string)
	if clientID == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operations")
	}
	return getFileTaskManager().create(clientID, fileTaskTypeMove, "Move files", ops), nil
}

func toModelFileTask(t *fileTask) *model.FileTask {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return &model.FileTask{
		ID:         t.ID,
		Type:       model.FileTaskType(t.Type),
		Title:      t.Title,
		Status:     model.FileTaskStatus(t.Status),
		Error:      t.Error,
		TotalBytes: int(t.TotalBytes),
		DoneBytes:  int(t.DoneBytes),
		TotalItems: int(t.TotalItems),
		DoneItems:  int(t.DoneItems),
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}
