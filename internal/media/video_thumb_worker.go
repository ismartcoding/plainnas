package media

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

type videoThumbReq struct {
	path    string
	w       int
	h       int
	quality int
	seekSec float64
}

var (
	videoThumbMgrOnce sync.Once
	videoThumbMgr     *cliqueue.Manager[videoThumbReq, []byte]
)

func getVideoThumbManager() *cliqueue.Manager[videoThumbReq, []byte] {
	videoThumbMgrOnce.Do(func() {
		if envTruthy("PLAINNAS_DISABLE_FFMPEG_BATCH") {
			return
		}

		batchSize := cliqueue.ParseEnvInt("PLAINNAS_FFMPEG_BATCH_SIZE", 5)
		if batchSize < 3 {
			batchSize = 3
		}
		if batchSize > 10 {
			batchSize = 10
		}

		workers := cliqueue.ParseEnvInt("PLAINNAS_FFMPEG_WORKERS", cliqueue.DefaultWorkers())
		if workers < 1 {
			workers = 1
		}
		if workers > 16 {
			workers = 16
		}

		queueSize := cliqueue.ParseEnvInt("PLAINNAS_FFMPEG_QUEUE", workers*batchSize*4)
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
				// Per-video timeout historically is ~6s; scale by batch size, cap to avoid long stalls.
				per := 7 * time.Second
				t := time.Duration(batchLen) * per
				if t < 6*time.Second {
					t = 6 * time.Second
				}
				if t > 60*time.Second {
					t = 60 * time.Second
				}
				return t
			},
		}

		videoThumbMgr = cliqueue.New(cfg, runFFmpegThumbBatch)
	})
	return videoThumbMgr
}

func runFFmpegThumbBatch(ctx context.Context, batch []videoThumbReq) []cliqueue.BatchResult[[]byte] {
	results := make([]cliqueue.BatchResult[[]byte], len(batch))

	tmpDir, err := os.MkdirTemp("", "plainnas-ffmpegbatch-")
	if err != nil {
		for i := range results {
			results[i] = cliqueue.BatchResult[[]byte]{Err: err}
		}
		return results
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-probesize", "256k",
		"-analyzeduration", "0",
	}

	for _, j := range batch {
		ss := fmt.Sprintf("%.3f", j.seekSec)
		args = append(args,
			"-ss", ss,
			"-noaccurate_seek",
			"-skip_frame", "nokey",
			"-i", j.path,
		)
	}

	for i, j := range batch {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("%d.webp", i))
		q := clamp(j.quality, 1, 100)
		args = append(args,
			"-map", fmt.Sprintf("%d:v:0", i),
			"-frames:v", "1",
			"-an", "-sn", "-dn",
			"-vf", ffmpegScaleFilter(j.w, j.h),
			"-c:v", "libwebp",
			"-q:v", strconv.Itoa(q),
			"-compression_level", "0",
			"-preset", "default",
			"-f", "webp",
			outPath,
		)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if err := cmd.Run(); err != nil {
		for i := range results {
			results[i] = cliqueue.BatchResult[[]byte]{Err: err}
		}
		return results
	}

	for i := range batch {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("%d.webp", i))
		b, err := os.ReadFile(outPath)
		if err != nil {
			results[i] = cliqueue.BatchResult[[]byte]{Err: err}
			continue
		}
		if len(b) == 0 {
			results[i] = cliqueue.BatchResult[[]byte]{Err: ErrNoCover}
			continue
		}
		results[i] = cliqueue.BatchResult[[]byte]{Res: b}
	}

	return results
}
