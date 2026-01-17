package db

// Persistent mapping between filesystem UUID (FSUUID) and assigned usbX slot.
// Stored in Pebble so mounts can be reproduced across restarts.

func storageFSUUIDSlotKey() string {
	return "storage:fsuuid_slot"
}

// GetFSUUIDSlotMap loads the persisted FSUUID -> usb slot mapping.
// Missing key returns an empty map.
func GetFSUUIDSlotMap() map[string]int {
	m := map[string]int{}
	_ = GetDefault().LoadJSON(storageFSUUIDSlotKey(), &m)
	return m
}

// StoreFSUUIDSlotMap persists the FSUUID -> usb slot mapping.
func StoreFSUUIDSlotMap(m map[string]int) error {
	if m == nil {
		m = map[string]int{}
	}
	return GetDefault().StoreJSON(storageFSUUIDSlotKey(), m)
}
