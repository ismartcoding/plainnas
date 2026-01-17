# Thumbnails & Cover Rules

This document describes PlainNAS thumbnail generation behavior and the cover-art selection policy for audio/video files.

## Thumbnail pipeline

PlainNAS generates thumbnails on demand (via the file-serving endpoint) and caches the resulting bytes in Pebble (prefix `thumb:`).

Generation order (WEBP-first):

- **Images / extracted covers**: Prefer `vipsthumbnail` (batched worker queue) → fallback to `vips` CLI
- **Videos without cover**: use `ffmpeg` to extract a single frame and encode to WEBP

### Image resizing via vipsthumbnail (batched worker queue)

To reduce `vipsthumbnail` process startup overhead under bursty requests, PlainNAS batches multiple resize jobs into a single `vipsthumbnail` invocation (grouped by target size and quality).

Tuning:

- `PLAINNAS_VIPS_BATCH_SIZE` (3..10, default 5)
- `PLAINNAS_VIPS_WORKERS` (default `min(CPU/2, 4)`)
- `PLAINNAS_VIPS_QUEUE` (default `workers * batchSize * 4`)

If `vipsthumbnail` is unavailable or fails, PlainNAS falls back to the `vips` CLI path.

### Video frame extraction (no embedded/sidecar cover)

When a video has no sidecar/embedded cover, PlainNAS generates a thumbnail by extracting **one** frame with `ffmpeg`.

Timepoint selection (seconds):

- duration > 10 minutes (600s): extract at 30s
- duration <= 10 minutes: extract at 5s
- duration < 5 seconds: extract the first frame (0s)

Performance requirements:

- Duration is determined via fast project-local sources (DB-cached metadata when available, otherwise pure-Go container probing for supported formats). PlainNAS intentionally avoids invoking `ffprobe` in the thumbnail hot path.
- `ffmpeg` uses **fast seek** by placing `-ss` **before** `-i`.
- Minimal work: single frame (`-frames:v 1`), no audio/subtitle/data (`-an -sn -dn`), and direct scaling via `-vf scale=...`.
- Single attempt only: no black-frame detection and no retries.

#### Amortized ffmpeg batching (engineering optimization)

To reduce `ffmpeg` process startup overhead under bursty thumbnail requests, PlainNAS can **batch** multiple video-thumbnail jobs into a single `ffmpeg` invocation.

Key points:

- `ffmpeg` is still used as a CLI.
- One `ffmpeg` process handles a **small batch** of independent videos (default batch size: 5) and then exits.
- A small worker pool runs these batches concurrently (default workers: `min(CPU/2, 4)`).
- The job queue is bounded to avoid unbounded I/O pressure; when the queue is full, thumbnail requests apply backpressure (wait) instead of spawning unlimited processes.

Tuning / disabling:

- `PLAINNAS_DISABLE_FFMPEG_BATCH=1`: disable batching and always use one `ffmpeg` process per video thumbnail.
- `PLAINNAS_FFMPEG_BATCH_SIZE` (3..10, default 5): maximum number of videos per `ffmpeg` invocation.
- `PLAINNAS_FFMPEG_WORKERS` (default `min(CPU/2, 4)`): number of concurrent batch workers.
- `PLAINNAS_FFMPEG_QUEUE` (default `workers * batchSize * 4`): bounded queue capacity.

Optional setting:

- `PLAINNAS_DISABLE_VIPS=1`: disable the libvips path (thumbnail generation may fail under WEBP-only policy).

Size cap:

- `MaxThumbBytes` (default: 200KB) is an upper bound for encoded thumbnail bytes.

## Output formats

- Thumbnails are **WEBP by default** (stored in Pebble cache prefix `thumb:`).
- **GIF exception**: if the effective thumbnail source is a GIF and:
  - file size < 5MB, and
  - max(width,height) < 600px
  then PlainNAS returns the **original GIF bytes** as the preview (keeps animation). Otherwise, it generates a WEBP thumbnail from the **first frame**.

## Cover extraction policy (audio/video)

Some audio/video containers can provide a cover image. PlainNAS selects a cover image using this priority:

1. **Sidecar image** (same directory)
2. **Embedded cover** (container metadata)

### Sidecar candidates

For a media file at `dir/<stem>.<ext>`, the following sidecar names are checked in order:

- `<stem>.jpg`, `<stem>.jpeg`, `<stem>.png`, `<stem>.gif`
- `<stem>.jpg`, `<stem>.jpeg`, `<stem>.webp`, `<stem>.png`, `<stem>.gif`
- `cover.jpg`, `cover.jpeg`, `cover.webp`, `cover.png`, `cover.gif`
- `folder.jpg`, `folder.jpeg`, `folder.webp`, `folder.png`, `folder.gif`

If a sidecar exists, it is used directly as the thumbnail source.

### Embedded cover support (pure Go)

- **MP3**: ID3v2 `APIC` frame
- **FLAC**: metadata block `PICTURE`
- **MP4/M4A/MOV/M4V/3GP**: `covr` atom (inside `moov`)

### Explicit non-goals

- Cover extraction (embedded artwork) remains pure-Go; however, **on-demand video thumbnails** use `ffmpeg` when no cover exists.

### Cache invalidation

If a sidecar cover exists, its `mtime/size` is used to invalidate the thumbnail cache key, so updating the sidecar refreshes thumbnails.

## No-cover behavior

- If an audio/video file has no cover (and video extraction fails), the API returns `404` and records a short fail-cache entry (prefix `thumbfail:`) keyed by `mtime/size`, so repeated requests don’t create retry storms.
