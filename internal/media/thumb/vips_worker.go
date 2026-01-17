package thumb

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"ismartcoding/plainnas/internal/pkg/cliqueue"
)

// VipsWorker batches vipsthumbnail jobs to amortize process startup costs.
//
// One vipsthumbnail process can accept multiple input files and produce one output per input.
// This worker groups jobs by (size, quality) and runs them in small batches.
type VipsWorker struct {
	maxWorkers int
	managers   sync.Map // key => *cliqueue.Manager[vipsReq, struct{}]
}

func NewVipsWorker(maxConcurrent int) *VipsWorker {
	if maxConcurrent <= 0 {
		maxConcurrent = cliqueue.DefaultWorkers()
	}
	return &VipsWorker{maxWorkers: maxConcurrent}
}

type vipsReq struct {
	input  string
	output string
}

// ResizeWEBP resizes `input` to `size` and writes WEBP bytes to `output`.
//
// It batches multiple jobs into a single vipsthumbnail invocation when possible.
func (w *VipsWorker) ResizeWEBP(ctx context.Context, input, output string, size int, quality int) error {
	if size <= 0 {
		return fmt.Errorf("invalid size: %d", size)
	}
	quality = cliqueue.ClampInt(quality, 1, 100)

	if _, err := exec.LookPath("vipsthumbnail"); err != nil {
		return err
	}

	key := fmt.Sprintf("s%d-q%d", size, quality)
	managerAny, ok := w.managers.Load(key)
	if !ok {
		batchSize := cliqueue.ClampInt(cliqueue.ParseEnvInt("PLAINNAS_VIPS_BATCH_SIZE", 5), 3, 10)
		workers := cliqueue.ClampInt(cliqueue.ParseEnvInt("PLAINNAS_VIPS_WORKERS", w.maxWorkers), 1, 16)
		queueSize := cliqueue.ParseEnvInt("PLAINNAS_VIPS_QUEUE", workers*batchSize*4)
		if queueSize < workers*batchSize {
			queueSize = workers * batchSize
		}
		if queueSize > 4096 {
			queueSize = 4096
		}

		cfg := cliqueue.Config{
			Workers:      workers,
			BatchSize:    batchSize,
			QueueSize:    queueSize,
			CoalesceWait: 2 * time.Millisecond,
			BatchTimeout: func(batchLen int) time.Duration {
				per := 10 * time.Second
				t := time.Duration(batchLen) * per
				if t < 10*time.Second {
					t = 10 * time.Second
				}
				if t > 60*time.Second {
					t = 60 * time.Second
				}
				return t
			},
		}

		created := cliqueue.New[vipsReq, struct{}](cfg, func(ctx context.Context, batch []vipsReq) []cliqueue.BatchResult[struct{}] {
			return runVipsthumbnailBatch(ctx, batch, size, quality)
		})
		managerAny, _ = w.managers.LoadOrStore(key, created)
	}

	m := managerAny.(*cliqueue.Manager[vipsReq, struct{}])
	_, err := m.Submit(ctx, vipsReq{input: input, output: output})
	return err
}

func runVipsthumbnailBatch(ctx context.Context, batch []vipsReq, size int, quality int) []cliqueue.BatchResult[struct{}] {
	results := make([]cliqueue.BatchResult[struct{}], len(batch))

	tmpDir, err := os.MkdirTemp("", "plainnas-vipsthumbnail-batch-")
	if err != nil {
		for i := range results {
			results[i] = cliqueue.BatchResult[struct{}]{Err: err}
		}
		return results
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	inputs := make([]string, 0, len(batch))
	for i, j := range batch {
		ext := filepath.Ext(j.input)
		name := fmt.Sprintf("in-%03d%s", i, ext)
		linkPath := filepath.Join(tmpDir, name)
		if err := os.Symlink(j.input, linkPath); err != nil {
			for k := range results {
				results[k] = cliqueue.BatchResult[struct{}]{Err: err}
			}
			return results
		}
		inputs = append(inputs, linkPath)
	}

	outPattern := filepath.Join(tmpDir, fmt.Sprintf("out-%%s.webp[Q=%d]", quality))
	args := make([]string, 0, len(inputs)+4)
	args = append(args, inputs...)
	args = append(args, "-s", strconv.Itoa(size), "-o", outPattern)

	cmd := exec.CommandContext(ctx, "vipsthumbnail", args...)
	if err := cmd.Run(); err != nil {
		for i := range results {
			results[i] = cliqueue.BatchResult[struct{}]{Err: err}
		}
		return results
	}

	for i, j := range batch {
		base := fmt.Sprintf("in-%03d", i)
		srcPath := filepath.Join(tmpDir, fmt.Sprintf("out-%s.webp", base))
		b, err := os.ReadFile(srcPath)
		if err != nil {
			results[i] = cliqueue.BatchResult[struct{}]{Err: err}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(j.output), 0755); err != nil {
			results[i] = cliqueue.BatchResult[struct{}]{Err: err}
			continue
		}
		if err := os.WriteFile(j.output, b, 0644); err != nil {
			results[i] = cliqueue.BatchResult[struct{}]{Err: err}
			continue
		}
		results[i] = cliqueue.BatchResult[struct{}]{Res: struct{}{}}
	}

	return results
}
