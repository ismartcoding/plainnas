package cache

import (
	"encoding/json"
	"fmt"
	"ismartcoding/plainnas/internal/db"
	"sync"
	"time"
)

var cacheOnce sync.Once

// CacheItem represents a cached item with expiration
type CacheItem struct {
	Value     []byte    `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GetCache returns a PebbleDB instance for caching
func GetCache() *db.PebbleDB {
	return db.GetDefault()
}

// Set stores a value in the cache with expiration
func Set(key string, value interface{}, expiration time.Duration) error {
	item := CacheItem{
		Value:     []byte(fmt.Sprintf("%v", value)),
		ExpiresAt: time.Now().Add(expiration),
	}

	return GetCache().StoreJSON(key, item)
}

// Get retrieves a value from the cache
func Get(key string) (string, error) {
	var item CacheItem
	err := GetCache().LoadJSON(key, &item)
	if err != nil {
		return "", err
	}

	if time.Now().After(item.ExpiresAt) {
		GetCache().DeleteByKey(key)
		return "", nil
	}

	return string(item.Value), nil
}

// Delete removes a value from the cache
func Delete(key string) error {
	return GetCache().DeleteByKey(key)
}

// SetJSON stores a JSON-serializable value in the cache with expiration
func SetJSON(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	item := CacheItem{
		Value:     data,
		ExpiresAt: time.Now().Add(expiration),
	}

	return GetCache().StoreJSON(key, item)
}

// GetJSON retrieves and deserializes a JSON value from the cache
func GetJSON(key string, value interface{}) error {
	var item CacheItem
	err := GetCache().LoadJSON(key, &item)
	if err != nil {
		return err
	}

	if time.Now().After(item.ExpiresAt) {
		GetCache().DeleteByKey(key)
		return nil
	}

	return json.Unmarshal(item.Value, value)
}
