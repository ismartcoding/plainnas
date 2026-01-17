# Media Items

This document explains how PlainNAS **media items** are stored, how their IDs (UUIDs) are generated, how indexing works, and which business logic / GraphQL entry points are related to media items.

Terminology:

- **media item**: a record represented by the Go struct `internal/media.MediaFile` (one file in the media library).
- **UUID**: the primary identifier for a media item.
- **Pebble**: the project’s KV store (see `internal/db`).
- **media search index**: the on-disk inverted index under `consts.DATA_DIR/searchidx_media` (custom mmap + postings).
- **type secondary indexes**: Pebble keys under the `media:type:` prefix that enable fast listing/sorting/filtering by type/trash/mtime/name/size.

---

## 1. Data model: `MediaFile`

Media items are stored in Pebble as JSON.

Key fields (see `internal/media/types.go`):

- `UUID`: primary key.
- `FSUUID/Ino/Ctime`: a file-identity tuple derived from the filesystem UUID + inode + ctime.
- `Path`: current physical path (when trashed, this becomes the trash path).
- `OriginalPath`: original path before moving to trash (used for restore and bucket grouping).
- `Name/Size/ModifiedAt/Type`: file name, size, mtime, inferred media type (audio/video/image/other).
- `DurationSec/DurationRefMod/DurationRefSize`: best-effort cached duration for audio/video.
- `IsTrash/TrashPath/DeletedAt`: trash state.

`Type` is inferred from the filename extension by `inferType()`.

---

## 2. How the ID (UUID) is generated

### 2.1 File identity source

On Linux, media-item UUID generation is based on:

- **filesystem UUID** (FSUUID): resolved from the mount’s block device via `/dev/disk/by-uuid` (best-effort)
- `ino`: inode number (`st.Ino`)
- `ctimeSec`: inode change time in seconds (`st.Ctim.Sec`)

Implementation: `internal/media/uuid_linux.go`.

### 2.2 UUID algorithm

`GenerateUUIDFromPath(path)` calls `uuidFromTriplet(fsUUID, ino, ctime)`:

1. Build a string: `"fsuuid:ino:ctime"`
2. Compute: `SHA1(namespace || tripletString)`
3. Take the first 16 bytes of the digest
4. Set RFC4122 variant bits and set version bits to 5
5. Format as `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

This behaves similarly to UUIDv5 conceptually, but it is implemented directly in code with a custom namespace and SHA1 truncation.

### 2.3 Is this UUID stable?

Stability depends on whether `fsuuid/ino/ctime` stays stable:

- Rename/move within the same filesystem: `fsuuid` and `ino` stay, but `ctime` can change (rename updates inode ctime), so the UUID may change.
- Copy: inode changes, UUID changes.
- Move across filesystems: `fsuuid` changes, UUID changes.
- Metadata changes (permissions/owner): can update `ctime`, UUID may change.

PlainNAS also persists `FSUUID/Ino/Ctime` and keeps an `FID -> UUID` mapping (below) to preserve UUIDs when a file identity is already known.

### 2.4 Why do we also call `FindUUIDByFID`

Both `UpsertPath()` and `ScanAndSync()` generate a UUID, then do:

- `ex := FindUUIDByFID(fsuuid, ino, ctime)`
- if `ex != "" && ex != id`, use `ex` instead

Call sites: `internal/media/api.go`, `internal/media/scan.go`.

This keeps the UUID stable for an already-known file identity (e.g., when historical data exists or if generation behavior changes).

---

## 3. Persistence layout (Pebble KV)

Media items store primary records and several secondary/lookup keys.

### 3.1 Primary record

- Key: `media:uuid:<uuid>`
- Value: JSON-encoded `MediaFile`

Write path: `internal/media/store.go` (`UpsertMedia()`).

### 3.2 Lookup mappings

These allow fast lookups from path or file identity:

- Path -> UUID
  - Key: `media:path:<path>`
  - Value: `<uuid>`

- FID -> UUID
  - Key: `media:fid:<hash(fsuuid)>:<ino>:<ctime>`
  - Value: `<uuid>`

Lookup helpers:

- `FindByPath(path)`
- `FindUUIDByFID(fsuuid, ino, ctime)`

### 3.3 Type secondary indexes (fast listing/sorting)

`UpsertMedia()` maintains keys under `media:type:` (empty values) to support fast iteration by `type + trash + sortKey`.

Examples:

- `media:type:audio:trash:0:mod:00000000017000000000:<uuid>`
- `media:type:audio:trash:0:moddesc:...:<uuid>`
- `media:type:audio:trash:0:name:<normalizedName>:<uuid>`
- `media:type:audio:trash:0:namedesc:<byteInvertedName>:<uuid>`
- `media:type:audio:trash:0:size:<paddedSize>:<uuid>`
- `media:type:audio:trash:0:sizedesc:<invertedSize>:<uuid>`

Implementation: `internal/media/type_index.go`.

On startup, `media.EnsureTypeIndexes()` verifies and rebuilds them if needed (entry point: `cmd/run.go`).

---

## 4. Indexing for media items (search + listing)

PlainNAS has two primary indexing mechanisms for media items:

1. **Type secondary indexes (inside Pebble)**: fast path for empty-query list/count/sort.
2. **On-disk inverted search index**: used by `media.Search()` for text search over name/path (exact tokens + fuzzy ngrams).

### 4.1 Type secondary indexes (fast path)

Typical usage: `internal/graph/helpers/media_helper.go` (`scanMedia()` / `CountMedia()`).

When `text == ""` and no `ids:` filter is present, the code can:

- Build a prefix with `media.TypeIndexPrefix(mediaType, trashOnly, idxKind)`
- Iterate with Pebble `Iterate(prefix, ...)` (natural key order)
- Extract UUID from the key suffix (`media.UUIDFromTypeIndexKey`)
- Load the full record via `media.GetFile(uuid)`

This avoids scanning and unmarshalling the full `media:uuid:` corpus.

### 4.2 Search inverted index (`searchidx_media`)

Index directory: `consts.DATA_DIR/searchidx_media`.

Exact index files:

- `name.dict.json`, `name.postings.dat`, `name.postings.idx`
- `path.dict.json`, `path.postings.dat`, `path.postings.idx`

Fuzzy ngram index files:

- `name_ngram.*`
- `path_ngram.*`

Build entry point: `internal/media/search_index.go` (`BuildMediaIndex()`).

Build summary:

- Iterate all `media:uuid:` records
- For each record:
  - `docID = xxhash64(UUID)` (posting document id)
  - Persist mapping: `media:docid:<docID> -> <uuid>`
  - Tokenize both `Name` and `Path`:
    - `tokenize()` produces exact tokens
    - `buildQueryNgrams()` produces fuzzy tokens (ASCII 2-grams; CJK bigrams)
- Write `term -> posting(docIDs)` to on-disk files (dict + dat + idx); query uses mmap.

Query entry point: `internal/media/search_index.go` (`Search(query, filters, offset, limit)`).

Query strategy:

- If query starts with `ids:`: load by UUID and apply filters.
- Otherwise:
  - If index files exist (`MediaIndexExists()`): use index-backed search
    - exact: union(name,path) postings per token, then intersect across tokens
    - fuzzy: intersect ngram postings, then union into the final set (with per-term caps)
    - map `docID -> uuid` via `media:docid`, then load records via `GetFile`
  - If index missing/fails: fallback by scanning `media:uuid:` and doing substring matching

Supported filters:

- `type`: audio/video/image/other
- `trash`: true/false
- `path_prefix`: can contain multiple prefixes separated by `|`

### 4.3 When indexes are built

- On startup: `cmd/services/watcher/run.go`
  - If `media.MediaIndexExists()` is false:
    - run `media.ScanAndSync(root)` for each storage volume mount point (populate Pebble)
    - then run `media.BuildMediaIndex()` (generate `searchidx_media`)

- Via GraphQL: `rebuildMediaIndex(root)` (`internal/graph/media_scan_api.go`)
  - Calls `ResetAllMediaData()` (clears Pebble media data and deletes the on-disk index directory)
  - Starts `ScanAndSync(root)` to repopulate Pebble
  - Note: the current implementation does not automatically call `BuildMediaIndex()` after scanning, so text search may temporarily use the fallback path until the index is rebuilt (e.g., on next watcher startup or via a manual rebuild step).

---

## 5. Media-item business logic (by feature)

### 5.1 Scan and sync

- `media.ScanAndSync(root)`: walks directories, upserts items, reports progress, and cleans up missing files.
- Progress event: `consts.EVENT_MEDIA_SCAN_PROGRESS` via `eventbus`.
- Source-dir whitelist: `db.GetMediaSourceDirs()`; when set, only paths under these prefixes are indexed.
- Explicitly skipped:
  - `.nas-trash` (unified trash directory)
  - system dirs like `/proc`, `/sys`, `/dev`, etc.
  - hidden directories/files (`.` prefix)

### 5.2 Single-file updates (watcher / uploads)

- `media.UpsertPath(path)`: builds a `MediaFile` and calls `UpsertMedia()`.
- `media.ScanFile(path)` / `media.ScanFiles(paths)`: “index immediately”; currently `FlushMediaIndexBatch()` is a no-op, so this does not incrementally update the on-disk inverted index.

Typical call sites:

- after copy/move: `internal/graph/files_copy_move_api.go`
- after upload merge: `internal/graph/upload_merge_chunks_api.go`

### 5.3 Trash / restore / delete

GraphQL batch actions: `trashMediaItems`, `restoreMediaItems`, `deleteMediaItems` (`internal/graph/media_items_actions_api.go`).

Implementation:

- `media.TrashUUID(uuid)`: uses `internal/fs.TrashPaths()` to move into `.nas-trash`, then updates:
  - `Path` -> trash path
  - `OriginalPath` preserved
  - `IsTrash=true`, `DeletedAt` set
  - persists via `UpsertMedia()`
- `media.RestoreUUID(uuid)`: uses `internal/fs.RestorePaths()` to restore
- `media.DeleteUUIDPermanently(uuid)`: deletes the file (special-cases `.nas-trash`), then calls `DeleteMedia(uuid)` to remove metadata and secondary keys

Note: `DeleteMedia()` does not delete per-document entries from `searchidx_media`; removals are reflected by rebuilding the on-disk search index (or by the fallback path scanning Pebble).

### 5.4 Listing, counting, sorting

GraphQL list/count for audios/videos/images is primarily implemented in `internal/graph/helpers/media_helper.go`:

- Empty query: prefer the `media:type:` fast path
- Text query: use `media.Search()` (index-backed if present; otherwise fallback)
- Sorting:
  - fast path is naturally sorted by key encoding (`moddesc`, `name`, `namedesc`, `size`, `sizedesc`, etc.)
  - fallback sorts in memory

### 5.5 Buckets (directory grouping)

- Bucket ID: FNV-1a 32-bit hash of the parent directory path (`helpers.bucketIDFromDir`)
- Bucket list: `internal/graph/media_buckets_api.go`
  - when mediaType is specified: iterates the `moddesc` index so `topItems` tend to be recent
  - for default: scans all `media:uuid:` (excluding trash)

### 5.6 Duration caching

- `media.EnsureDuration(mf)`: best-effort extracts audio/video duration, caches into `DurationSec`, and persists via `UpsertMedia()`.
- In list views, duration probing is deferred to only the final paginated items to avoid expensive full-corpus probing.

### 5.7 Encrypted “fileId” for URLs (not the UUID)

`internal/graph/helpers/media_helper.go` provides `GenerateEncryptedFileID(path)`:

- Uses the global `urlToken` as the key (`db.EnsureURLToken()` ensures it exists)
- Encrypts the file path with ChaCha20 and returns a base64 string

This is an opaque ID for sharing/URLs; it is not the media item UUID.

---

## 6. Troubleshooting

### 6.1 Search is slow / fallback is used

Likely cause: `searchidx_media` is missing or corrupted, so `media.MediaIndexExists()` returns false.

What to do:

- Restart the service so watcher startup rebuilds it (`cmd/services/watcher/run.go`), or
- Trigger GraphQL `rebuildMediaIndex(root)` and then ensure the on-disk search index gets rebuilt (the current rebuild path repopulates Pebble first and may not immediately regenerate `searchidx_media`).

### 6.2 Listing/sorting/counting is not using the fast path

Likely cause: missing `media:type:` secondary indexes.

- Startup calls `media.EnsureTypeIndexes()` to repair them.
- In development, deleting the DB and rebuilding is acceptable per project workflow.

---

## 7. Quick reference (code entry points)

- Data model: `internal/media/types.go`
- UUID generation (Linux): `internal/media/uuid_linux.go`
- Upsert/Delete/mappings: `internal/media/store.go`, `internal/media/keys.go`
- Scan/sync: `internal/media/scan.go`, `internal/media/control.go`
- Trash: `internal/media/trash.go`
- Type secondary indexes: `internal/media/type_index.go`
- Search inverted index: `internal/media/search_index.go`
- GraphQL rebuild/scan: `internal/graph/media_scan_api.go`
- GraphQL batch actions: `internal/graph/media_items_actions_api.go`
- GraphQL list/count/sort: `internal/graph/helpers/media_helper.go`
- Buckets: `internal/graph/media_buckets_api.go`
