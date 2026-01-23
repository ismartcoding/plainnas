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

func webpSaveOptions(quality int) string {
	q := cliqueue.ClampInt(quality, 1, 100)
	// libvips webpsave: effort 0..6. Keep default low for interactive thumbnails.
	effort := cliqueue.ClampInt(cliqueue.ParseEnvInt("PLAINNAS_WEBP_EFFORT", 0), 0, 6)
	return fmt.Sprintf("[Q=%d,effort=%d,strip]", q, effort)
}

func vipsCLIConcurrencyArg() string {
	// This sets libvips' internal thread pool size for this process.
	// Default 1: often best for latency on modest CPUs.
	n := cliqueue.ClampInt(cliqueue.ParseEnvInt("PLAINNAS_VIPS_CONCURRENCY", 1), 1, 16)
	return fmt.Sprintf("--vips-concurrency=%d", n)
}

// VipsWorker batches vipsthumbnail jobs to amortize process startup costs.
//
// One vipsthumbnail process can accept multiple input files and produce one output per input.
// This worker groups jobs by (size, quality) and runs them in small batches.
type VipsWorker struct {
	maxWorkers int
	managers   sync.Map // key => *cliqueue.Manager[vipsReq, struct{}]
	bytesMgrs  sync.Map // key => *cliqueue.Manager[vipsReqBytes, []byte]
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

type vipsReqBytes struct {
	input string
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

// ResizeWEBPBytes resizes `input` to `size` and returns WEBP bytes.
//
// This avoids extra disk IO for callers that only need the bytes (e.g. HTTP thumbnails).
func (w *VipsWorker) ResizeWEBPBytes(ctx context.Context, input string, size int, quality int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid size: %d", size)
	}
	quality = cliqueue.ClampInt(quality, 1, 100)

	if _, err := exec.LookPath("vipsthumbnail"); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("s%d-q%d", size, quality)
	managerAny, ok := w.bytesMgrs.Load(key)
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

		created := cliqueue.New[vipsReqBytes, []byte](cfg, func(ctx context.Context, batch []vipsReqBytes) []cliqueue.BatchResult[[]byte] {
			return runVipsthumbnailBatchBytes(ctx, batch, size, quality)
		})
		managerAny, _ = w.bytesMgrs.LoadOrStore(key, created)
	}

	m := managerAny.(*cliqueue.Manager[vipsReqBytes, []byte])
	res, err := m.Submit(ctx, vipsReqBytes{input: input})
	if err != nil {
		return nil, err
	}
	return res, nil
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

	// Fast-path: for a single item, avoid symlinks and %s output patterns.
	if len(batch) == 1 {
		outPath := filepath.Join(tmpDir, "out.webp")
		outArg := outPath + webpSaveOptions(quality)
		args := []string{vipsCLIConcurrencyArg(), batch[0].input, "-s", strconv.Itoa(size), "-o", outArg}
		cmd := exec.CommandContext(ctx, "vipsthumbnail", args...)
		if err := cmd.Run(); err != nil {
			results[0] = cliqueue.BatchResult[struct{}]{Err: err}
			return results
		}
		b, err := os.ReadFile(outPath)
		if err != nil {
			results[0] = cliqueue.BatchResult[struct{}]{Err: err}
			return results
		}
		if err := os.MkdirAll(filepath.Dir(batch[0].output), 0755); err != nil {
			results[0] = cliqueue.BatchResult[struct{}]{Err: err}
			return results
		}
		if err := os.WriteFile(batch[0].output, b, 0644); err != nil {
			results[0] = cliqueue.BatchResult[struct{}]{Err: err}
			return results
		}
		results[0] = cliqueue.BatchResult[struct{}]{Res: struct{}{}}
		return results
	}

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

	outPattern := filepath.Join(tmpDir, fmt.Sprintf("out-%%s.webp%s", webpSaveOptions(quality)))
	args := make([]string, 0, len(inputs)+5)
	args = append(args, vipsCLIConcurrencyArg())
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

func runVipsthumbnailBatchBytes(ctx context.Context, batch []vipsReqBytes, size int, quality int) []cliqueue.BatchResult[[]byte] {
	results := make([]cliqueue.BatchResult[[]byte], len(batch))

	tmpDir, err := os.MkdirTemp("", "plainnas-vipsthumbnail-batch-")
	if err != nil {
		for i := range results {
			results[i] = cliqueue.BatchResult[[]byte]{Err: err}
		}
		return results
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Fast-path: for a single item, avoid symlinks and %s output patterns.
	if len(batch) == 1 {
		outPath := filepath.Join(tmpDir, "out.webp")
		outArg := outPath + webpSaveOptions(quality)
		args := []string{vipsCLIConcurrencyArg(), batch[0].input, "-s", strconv.Itoa(size), "-o", outArg}
		cmd := exec.CommandContext(ctx, "vipsthumbnail", args...)
		if err := cmd.Run(); err != nil {
			results[0] = cliqueue.BatchResult[[]byte]{Err: err}
			return results
		}
		b, err := os.ReadFile(outPath)
		if err != nil {
			results[0] = cliqueue.BatchResult[[]byte]{Err: err}
			return results
		}
		results[0] = cliqueue.BatchResult[[]byte]{Res: b}
		return results
	}

	inputs := make([]string, 0, len(batch))
	for i, j := range batch {
		ext := filepath.Ext(j.input)
		name := fmt.Sprintf("in-%03d%s", i, ext)
		linkPath := filepath.Join(tmpDir, name)
		if err := os.Symlink(j.input, linkPath); err != nil {
			for k := range results {
				results[k] = cliqueue.BatchResult[[]byte]{Err: err}
			}
			return results
		}
		inputs = append(inputs, linkPath)
	}

	outPattern := filepath.Join(tmpDir, fmt.Sprintf("out-%%s.webp%s", webpSaveOptions(quality)))
	args := make([]string, 0, len(inputs)+5)
	args = append(args, vipsCLIConcurrencyArg())
	args = append(args, inputs...)
	args = append(args, "-s", strconv.Itoa(size), "-o", outPattern)

	cmd := exec.CommandContext(ctx, "vipsthumbnail", args...)
	if err := cmd.Run(); err != nil {
		for i := range results {
			results[i] = cliqueue.BatchResult[[]byte]{Err: err}
		}
		return results
	}

	for i := range batch {
		base := fmt.Sprintf("in-%03d", i)
		srcPath := filepath.Join(tmpDir, fmt.Sprintf("out-%s.webp", base))
		b, err := os.ReadFile(srcPath)
		if err != nil {
			results[i] = cliqueue.BatchResult[[]byte]{Err: err}
			continue
		}
		results[i] = cliqueue.BatchResult[[]byte]{Res: b}
	}

	return results
}
