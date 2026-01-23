package db

// Persistent mapping between storage volume ID and user-defined alias.
// Stored in Pebble as a JSON map.

import "encoding/json"

func storageVolumeAliasKey() string {
	return "storage:volume_alias"
}

// GetVolumeAliasMap loads the persisted ID -> alias mapping.
// Missing key returns an empty map.
func GetVolumeAliasMap() map[string]string {
	m := map[string]string{}
	_ = GetDefault().LoadJSON(storageVolumeAliasKey(), &m)
	return m
}

// StoreVolumeAliasMap persists the ID -> alias mapping.
func StoreVolumeAliasMap(m map[string]string) error {
	if m == nil {
		m = map[string]string{}
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	// This is user-facing configuration; make it durable immediately.
	return GetDefault().Set([]byte(storageVolumeAliasKey()), data, syncWriteOptions())
}

// SetVolumeAlias updates the alias for a given volume ID.
// If alias is empty after trimming, the mapping is removed.
func SetVolumeAlias(id string, alias string) error {
	m := GetVolumeAliasMap()
	if alias == "" {
		delete(m, id)
	} else {
		m[id] = alias
	}
	return StoreVolumeAliasMap(m)
}
