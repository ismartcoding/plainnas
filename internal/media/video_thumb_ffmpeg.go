package media

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

func generateVideoThumbnailWEBP(path string, w, h, quality int) ([]byte, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("media: ffmpeg not available")
	}

	// Smart timepoint selection to avoid intro black frames/logos.
	//
	// Rules:
	//   - duration > 10min (600s): 30s
	//   - duration <= 10min: 5s
	//   - duration < 5s: first frame (0s)
	//	Performance: we rely on fast seek by putting `-ss` before `-i`.
	seekSec := 5.0
	if dur, ok := fastDurationSeconds(path); ok && dur > 0 {
		if dur < 5 {
			seekSec = 0
		} else if dur > 600 {
			seekSec = 30
		}
		// Guardrail: avoid seeking at/after EOF.
		if seekSec >= dur {
			seekSec = 0
		}
	}

	q := clamp(quality, 1, 100)
	// Amortized path: batch multiple video thumbnail jobs into a single ffmpeg invocation.
	if mgr := getVideoThumbManager(); mgr != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		b, err := mgr.Submit(ctx, videoThumbReq{path: path, w: w, h: h, quality: q, seekSec: seekSec})
		if err != nil {
			return nil, err
		}
		if len(b) == 0 {
			return nil, ErrNoCover
		}
		return b, nil
	}

	b, err := ffmpegExtractKeyframeWebp(path, seekSec, w, h, q)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, ErrNoCover
	}
	return b, nil
}

// fastDurationSeconds returns duration seconds using project-local fast paths.
//
// Order:
//  1. DB cached MediaFile.DurationSec when valid
//  2. Pure-Go container probe (currently MP4/MOV/M4A/3GP)
//
// Intentionally does NOT shell out to ffprobe (too slow on some systems).
func fastDurationSeconds(path string) (float64, bool) {
	if dur, ok := cachedDurationSecondsByPath(path); ok {
		return dur, true
	}
	// Best-effort fallback: pure-Go duration probing for supported containers.
	if d, err := ProbeDurationSec(path); err == nil && d > 0 {
		return float64(d), true
	}
	return 0, false
}

func cachedDurationSecondsByPath(path string) (float64, bool) {
	uuid, err := FindByPath(path)
	if err != nil || uuid == "" {
		return 0, false
	}
	mf, err := GetFile(uuid)
	if err != nil || mf == nil {
		return 0, false
	}
	// Cached and still valid.
	if mf.DurationSec > 0 && mf.DurationRefMod == mf.ModifiedAt && mf.DurationRefSize == mf.Size {
		return float64(mf.DurationSec), true
	}
	return 0, false
}

func ffmpegExtractKeyframeWebp(path string, tSec float64, w, h, quality int) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	q := clamp(quality, 1, 100)
	ss := fmt.Sprintf("%.3f", tSec)

	// Performance notes:
	// - `-ss` before `-i` enables fast seek.
	// - `-noaccurate_seek` avoids decoding to the exact timestamp.
	// - Reduce probing to minimize startup latency.
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-probesize", "256k",
		"-analyzeduration", "0",
		"-ss", ss,
		"-noaccurate_seek",
		"-skip_frame", "nokey",
		"-i", path,
		"-frames:v", "1",
		"-an", "-sn", "-dn",
		"-vf", ffmpegScaleFilter(w, h),
		"-c:v", "libwebp",
		"-q:v", strconv.Itoa(q),
		"-compression_level", "0",
		"-preset", "default",
		"-f", "webp",
		"pipe:1",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	// Discard stderr except for debugging; errors are signaled by exit status.
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	if len(b) == 0 {
		return nil, ErrNoCover
	}
	return b, nil
}

func ffmpegScaleFilter(w, h int) string {
	if w > 0 && h > 0 {
		return fmt.Sprintf("scale=%d:%d:flags=fast_bilinear:force_original_aspect_ratio=decrease", w, h)
	}
	if w > 0 {
		return fmt.Sprintf("scale=%d:-1:flags=fast_bilinear", w)
	}
	if h > 0 {
		return fmt.Sprintf("scale=-1:%d:flags=fast_bilinear", h)
	}
	// As a last resort, keep original dimensions.
	return "scale=iw:ih:flags=fast_bilinear"
}
