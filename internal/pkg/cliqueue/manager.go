package cliqueue

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"time"
)

type BatchResult[Res any] struct {
	Res Res
	Err error
}

type BatchFunc[Req any, Res any] func(ctx context.Context, batch []Req) []BatchResult[Res]

type Config struct {
	Workers      int
	BatchSize    int
	QueueSize    int
	CoalesceWait time.Duration
	BatchTimeout func(batchLen int) time.Duration
}

type job[Req any, Res any] struct {
	req    Req
	respCh chan BatchResult[Res]
}

type Manager[Req any, Res any] struct {
	cfg  Config
	run  BatchFunc[Req, Res]
	jobs chan job[Req, Res]
	once sync.Once
}

func DefaultWorkers() int {
	// Suggested: min(CPU/2, 4)
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	return n
}

func New[Req any, Res any](cfg Config, run BatchFunc[Req, Res]) *Manager[Req, Res] {
	if run == nil {
		panic("cliqueue: run func is nil")
	}
	if cfg.Workers <= 0 {
		cfg.Workers = DefaultWorkers()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 5
	}
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 1
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = cfg.Workers * cfg.BatchSize * 4
	}
	if cfg.QueueSize < cfg.Workers*cfg.BatchSize {
		cfg.QueueSize = cfg.Workers * cfg.BatchSize
	}
	if cfg.CoalesceWait <= 0 {
		cfg.CoalesceWait = 2 * time.Millisecond
	}
	if cfg.BatchTimeout == nil {
		cfg.BatchTimeout = func(batchLen int) time.Duration {
			// Conservative default: 10s per task, capped at 60s.
			if batchLen < 1 {
				batchLen = 1
			}
			t := time.Duration(batchLen) * 10 * time.Second
			if t > 60*time.Second {
				t = 60 * time.Second
			}
			return t
		}
	}

	m := &Manager[Req, Res]{
		cfg:  cfg,
		run:  run,
		jobs: make(chan job[Req, Res], cfg.QueueSize),
	}
	m.once.Do(func() {
		for i := 0; i < m.cfg.Workers; i++ {
			go m.workerLoop()
		}
	})
	return m
}

func (m *Manager[Req, Res]) Submit(ctx context.Context, req Req) (Res, error) {
	var zero Res
	if ctx == nil {
		return zero, errors.New("cliqueue: nil context")
	}

	respCh := make(chan BatchResult[Res], 1)
	j := job[Req, Res]{req: req, respCh: respCh}

	select {
	case m.jobs <- j:
	case <-ctx.Done():
		return zero, ctx.Err()
	}

	select {
	case res := <-respCh:
		return res.Res, res.Err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (m *Manager[Req, Res]) workerLoop() {
	for {
		first := <-m.jobs

		batch := make([]job[Req, Res], 0, m.cfg.BatchSize)
		batch = append(batch, first)

		deadline := time.NewTimer(m.cfg.CoalesceWait)
		for len(batch) < m.cfg.BatchSize {
			select {
			case j := <-m.jobs:
				batch = append(batch, j)
			case <-deadline.C:
				deadline.Stop()
				goto RUN
			}
		}
		deadline.Stop()

	RUN:
		m.runBatch(batch)
	}
}

func (m *Manager[Req, Res]) runBatch(batch []job[Req, Res]) {
	reqs := make([]Req, 0, len(batch))
	for _, j := range batch {
		reqs = append(reqs, j.req)
	}

	timeout := m.cfg.BatchTimeout(len(batch))
	if timeout <= 0 {
		timeout = 1 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var results []BatchResult[Res]
	func() {
		defer func() {
			if r := recover(); r != nil {
				var zero Res
				results = make([]BatchResult[Res], len(batch))
				err := errors.New("cliqueue: batch runner panicked")
				for i := range results {
					results[i] = BatchResult[Res]{Res: zero, Err: err}
				}
			}
		}()
		results = m.run(ctx, reqs)
	}()

	// Normalize length: if runner returned fewer results, treat missing as failure.
	if len(results) != len(batch) {
		var zero Res
		err := errors.New("cliqueue: batch runner returned wrong result length")
		fixed := make([]BatchResult[Res], len(batch))
		for i := range fixed {
			fixed[i] = BatchResult[Res]{Res: zero, Err: err}
		}
		results = fixed
	}

	for i, j := range batch {
		j.respCh <- results[i]
	}
}
