# Performance Benchmarks

PlainNAS includes a built-in benchmark command to help evaluate performance and to make results easy to share with other developers.

## What it measures

`plainnas bench` runs three practical benchmarks:

- **Index build**: scans a dataset and builds the on-disk search index (and Pebble metadata).
- **Search queries**: executes repeated `SearchIndex` calls and reports latency (avg/p50/p95).
- **Thumbnail generation**: generates WEBP thumbnails for many images (through the same `vipsthumbnail` path used in production).

It also prints **RSS** (process resident memory) and **Go heap** before/after each phase.

## Requirements

- Linux (PlainNAS is Linux-only).
- For thumbnail benchmarks: `vipsthumbnail` must be installed.
  - On Debian/Ubuntu: `sudo apt install libvips-tools`

If `vipsthumbnail` is not available, the thumbnail phase is skipped (and the command prints a warning).

## Run

### Quick run (auto-generated dataset)

```bash
sudo go run ./main.go bench
```

Notes:
- By default the command generates a temporary dataset and uses a temporary data directory, so it will not touch `/var/lib/plainnas`.
- Use `--keep` to keep the temp dirs so you can inspect index sizes.

### Run on a real dataset

```bash
sudo go run ./main.go bench --dataset /mnt/storage
```

### JSON output (for pasting into issues/PRs)

```bash
sudo go run ./main.go bench --json > bench.json
```

## Useful flags

- `--files`, `--images`: control generated dataset size.
- `--thumb-size`, `--thumb-quality`, `--thumb-concurrency`: control thumbnail workload.
- `--queries`, `--limit`: control search workload.
- `--show-hidden`: include dotfiles in indexing.

## Interpreting results

The key numbers for presenting value are typically:

- **Index build ops/s**: how fast the index builds for large datasets.
- **Search p95**: tail latency for interactive search.
- **Thumbnails ops/s**: throughput for generating image thumbnails.

When comparing against other projects, try to keep the same:

- Dataset (file count, file name patterns, image sizes)
- Storage hardware (SSD/HDD)
- CPU / memory configuration
- `vipsthumbnail` version (thumbnail performance depends on libvips)
