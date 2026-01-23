package db

import (
	"encoding/json"
	"errors"
	"ismartcoding/plainnas/internal/consts"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/cockroachdb/pebble"
)

func syncWriteOptions() *pebble.WriteOptions {
	return &pebble.WriteOptions{Sync: true}
}

// PebbleDB wraps the pebble.DB type for better type safety
type PebbleDB struct {
	db *pebble.DB
}

// ErrIterateStop is a sentinel error that signals PebbleDB.Iterate to stop early
// without treating it as an error.
var ErrIterateStop = errors.New("iterate stop")

var (
	dbMain *PebbleDB
	once   sync.Once
)

func dbPath() string {
	return filepath.Join(consts.DATA_DIR, "pebble")
}

// GetDefault returns the default PebbleDB instance
func GetDefault() *PebbleDB {
	once.Do(func() {
		dbPath := dbPath()

		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			log.Panicf("error creating database directory: %v", err)
		}

		opts := &pebble.Options{
			Cache:                 pebble.NewCache(512 << 20), // 512MB
			MaxOpenFiles:          1000,
			L0CompactionThreshold: 8,
			Logger:                nil,
			DisableWAL:            false,
			MemTableSize:          256 << 20, // 256MB
		}
		defer opts.Cache.Unref()

		log.Printf("Opening PebbleDB at: %s", dbPath)
		db, err := pebble.Open(dbPath, opts)
		if err != nil {
			log.Panicf("error opening pebble db: %v", err)
		}

		dbMain = &PebbleDB{db: db}
		log.Printf("PebbleDB opened successfully")
	})
	return dbMain
}

// Close closes the database
func (p *PebbleDB) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Set stores a key-value pair
func (p *PebbleDB) Set(key []byte, value []byte, opts *pebble.WriteOptions) error {
	if opts == nil {
		opts = &pebble.WriteOptions{Sync: false}
	}
	return p.db.Set(key, value, opts)
}

// Get retrieves the value for a key
func (p *PebbleDB) Get(key []byte) ([]byte, error) {
	value, closer, err := p.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if closer == nil {
		return nil, nil
	}
	defer closer.Close()
	// Make a copy of the value since the original slice is invalid after closer.Close()
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Delete removes a key-value pair
func (p *PebbleDB) Delete(key []byte) error {
	// 使用异步删除模式
	return p.db.Delete(key, &pebble.WriteOptions{Sync: false})
}

// Iterate iterates over key-value pairs with a given prefix
func (p *PebbleDB) Iterate(prefix []byte, fn func(key []byte, value []byte) error) error {
	iter, err := p.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		if !hasPrefix(key, prefix) {
			break
		}
		value := iter.Value()

		// Make copies of key and value since they're only valid until the next iteration
		keyCopy := make([]byte, len(key))
		valueCopy := make([]byte, len(value))
		copy(keyCopy, key)
		copy(valueCopy, value)

		if err := fn(keyCopy, valueCopy); err != nil {
			if errors.Is(err, ErrIterateStop) {
				return nil
			}
			return err
		}
	}
	return iter.Error()
}

func nextPrefix(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	out := make([]byte, len(prefix))
	copy(out, prefix)
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] != 0xFF {
			out[i]++
			return out[:i+1]
		}
	}
	// prefix is all 0xFF; append a 0 byte to create an upper bound greater than any key with this prefix
	return append(out, 0)
}

// IterateReverse iterates over key-value pairs with a given prefix in reverse lexicographic order.
func (p *PebbleDB) IterateReverse(prefix []byte, fn func(key []byte, value []byte) error) error {
	upper := nextPrefix(prefix)
	iter, err := p.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: upper,
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.Last(); iter.Valid(); iter.Prev() {
		key := iter.Key()
		if !hasPrefix(key, prefix) {
			break
		}
		value := iter.Value()
		keyCopy := make([]byte, len(key))
		valueCopy := make([]byte, len(value))
		copy(keyCopy, key)
		copy(valueCopy, value)

		if err := fn(keyCopy, valueCopy); err != nil {
			if errors.Is(err, ErrIterateStop) {
				return nil
			}
			return err
		}
	}
	return iter.Error()
}

// StoreJSON stores a JSON-serializable value with a string key
func (p *PebbleDB) StoreJSON(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return p.Set([]byte(key), data, nil)
}

// LoadJSON retrieves and deserializes a JSON value
func (p *PebbleDB) LoadJSON(key string, value interface{}) error {
	data, err := p.Get([]byte(key))
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	return json.Unmarshal(data, value)
}

// DeleteByKey deletes a value by string key
func (p *PebbleDB) DeleteByKey(key string) error {
	return p.Delete([]byte(key))
}

// BatchDelete deletes keys in batches for higher throughput
func (p *PebbleDB) BatchDelete(keys [][]byte) error {
	if len(keys) == 0 {
		return nil
	}
	b := p.db.NewBatch()
	for _, k := range keys {
		if err := b.Delete(k, &pebble.WriteOptions{Sync: false}); err != nil {
			_ = b.Close()
			return err
		}
	}
	if err := b.Commit(&pebble.WriteOptions{Sync: false}); err != nil {
		_ = b.Close()
		return err
	}
	return b.Close()
}

// hasPrefix checks if bytes has prefix
func hasPrefix(s, prefix []byte) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}

func SetValue(key string, value string) {
	GetDefault().Set([]byte(key), []byte(value), &pebble.WriteOptions{Sync: true})
}

func DeleteValue(key string) {
	GetDefault().DeleteByKey(key)
}

func DeleteByPrefix(prefix string) {
	GetDefault().Iterate([]byte(prefix), func(key []byte, value []byte) error {
		GetDefault().DeleteByKey(string(key))
		return nil
	})
}

func GetMapByPrefix(prefix string) map[string]string {
	var mapItems = make(map[string]string)
	GetDefault().Iterate([]byte(prefix), func(key []byte, value []byte) error {
		mapItems[string(key)] = string(value)
		return nil
	})
	return mapItems
}
