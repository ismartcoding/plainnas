package consts

const (
	ETC_MAIN_CONFIG    = "/etc/plainnas/config.toml"
	ETC_TLS_SERVER_PEM = "/etc/plainnas/tls.pem"
	ETC_TLS_SERVER_KEY = "/etc/plainnas/tls.key"

	EVENT_SERVICE_STATE_CHANGED = "service:state:changed"

	EVENT_MEDIA_SCAN_PROGRESS = "media:scan:progress"
	EVENT_FILE_TASK_PROGRESS  = "file:task:progress"

	// Indexing and scanning performance constants
	SCAN_YIELD_EVERY_N   = 500
	SCAN_YIELD_MS        = 5
	ENABLE_SCAN_PRECOUNT = true
	SCAN_PIPELINE_BUFFER = 1024
	SCAN_INDEXER_WORKERS = 2
)

// DATA_DIR is the base directory for PlainNAS runtime data (Pebble DB, indexes, etc.).
// It is a var (not const) so tests can override it.
var DATA_DIR = "/var/lib/plainnas"
