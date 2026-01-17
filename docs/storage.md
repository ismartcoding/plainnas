# Storage & Indexing

This document describes PlainNAS data storage layout and the search indexing design.

## Data directory

All paths below are under the data directory `~/.plainnas/data` (see constants for defaults).

## Database (Pebble)

- Engine: CockroachDB Pebble (embedded KV)
- Path: `~/.plainnas/data/pebble`
- Open/init logic: see [internal/db/db.go](internal/db/db.go)
- Usage: sessions and tokens, recents, tags, and media metadata mappings (UUID/path/FID lookup). Relevant files:
	- Sessions: [internal/db/session.go](internal/db/session.go)
	- Tags: [internal/db/tag.go](internal/db/tag.go)
	- Recents: [internal/db/recent.go](internal/db/recent.go)
	- URL Token: [internal/db/urltoken.go](internal/db/urltoken.go)
	- Media mappings: [internal/media/store.go](internal/media/store.go)

## Cache (memory + Pebble)

- In-memory cache: `patrickmn/go-cache`, default 5m TTL. See [internal/cache/memory.go](internal/cache/memory.go)
- Persistent cache: simple TTL wrapper backed by Pebble (shared DB). See [internal/cache/pebble.go](internal/cache/pebble.go)
- Thumbnails: generated on demand and cached in Pebble using a content-derived key (prefix `thumb:`) composed from path, size, quality, and file metadata.
	- Read/write: [cmd/services/api/fs.go](cmd/services/api/fs.go)
	- Generation and cache key helper: [internal/media/thumbnail.go](internal/media/thumbnail.go)
	- Thumbnails are not stored as separate files on disk.

## File index (custom inverted index)

- Path/name index: `~/.plainnas/data/searchidx` (append-only build, mmap read-only). See [internal/search/fs_index.go](internal/search/fs_index.go)
- Fuzzy support: additional ngram dictionaries and postings for name/path (`name_ngram.*`, `path_ngram.*`) enabling ASCII 2-gram and CJK bigram fallback.
- Filters: bitmap-style postings for ext/size/mtime (`filter.*`) applied after intersections; pagination only on final IDs.
- Structure: term dictionaries (JSON), postings (`*.postings.dat`) + offsets (`*.postings.idx`). Name and path tokens stored separately.
- Query semantics: plain text queries search basenames only. For queries containing `/`: if it is an absolute path and exists on disk, it is resolved via the filesystem first (file -> `[file]`, dir -> direct children). Otherwise it falls back to the path index.
- Principles: Pebble is source of truth; index is discardable/rebuildable; search uses mmap without locks.
- Performance knobs: `SCAN_INDEXER_WORKERS`, `SCAN_PIPELINE_BUFFER`.
