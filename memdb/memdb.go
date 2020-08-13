/*
 * Copyright 2020 Saffat Technologies, Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package memdb

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/unit-io/unitdb/hash"
)

const (
	nShards = 32

	drainInterval         = 1 * time.Second
	memShrinkFactor       = 0.7
	dataTableShrinkFactor = 0.33 // shrinker try to free 33% of total memdb size
)

// To avoid lock bottlenecks block cache is divided into several (nShards) shards.
type blockCache []*memCache

type memCache struct {
	data         dataTable
	freeOffset   int64            // mem cache keep lowest offset that can be free.
	m            map[uint64]int64 // map[key]offset
	sync.RWMutex                  // Read Write mutex, guards access to internal map.
}

// newBlockCache creates a new concurrent block cache.
func newBlockCache() blockCache {
	m := make(blockCache, nShards)
	for i := 0; i < nShards; i++ {
		m[i] = &memCache{data: dataTable{}, m: make(map[uint64]int64)}
	}
	return m
}

// DB represents the block cache mem store.
// All DB methods are safe for concurrent use by multiple goroutines.
type DB struct {
	targetSize int64
	// block cache
	consistent *hash.Consistent
	blockCache blockCache

	// close
	closeW sync.WaitGroup
	closeC chan struct{}
}

// Open opens or creates a new DB of given size.
func Open(memSize int64) (*DB, error) {
	db := &DB{
		blockCache: newBlockCache(),
		// Close
		closeC: make(chan struct{}),
	}

	db.consistent = hash.InitConsistent(int(nShards), int(nShards))

	db.drain(drainInterval)

	return db, nil
}

func (db *DB) drain(interval time.Duration) {
	shrinkerTicker := time.NewTicker(interval)
	go func() {
		db.closeW.Add(1)
		defer func() {
			shrinkerTicker.Stop()
			db.closeW.Done()
		}()
		for {
			select {
			case <-db.closeC:
				return
			case <-shrinkerTicker.C:
				memSize, err := db.Size()
				if err == nil && float64(memSize) > float64(db.targetSize)*memShrinkFactor {
					db.shrinkDataTable()
				}
			}
		}
	}()
}

func (db *DB) shrinkDataTable() error {
	for i := 0; i < nShards; i++ {
		cache := db.blockCache[i]
		cache.Lock()
		if cache.freeOffset > 0 {
			if err := cache.data.shrink(cache.freeOffset); err != nil {
				cache.Unlock()
				return err
			}
		}
		for seq, off := range cache.m {
			if off < cache.freeOffset {
				delete(cache.m, seq)
			} else {
				cache.m[seq] = off - cache.freeOffset
			}
		}
		cache.freeOffset = 0
		cache.Unlock()
	}

	return nil
}

// Close closes the memdb.
func (db *DB) Close() error {
	// Signal all goroutines.
	close(db.closeC)

	for i := 0; i < nShards; i++ {
		cache := db.blockCache[i]
		cache.Lock()
		if err := cache.data.close(); err != nil {
			cache.Unlock()
			return err
		}
		cache.Unlock()
	}

	// Wait for all goroutines to exit.
	db.closeW.Wait()
	return nil
}

// getCache returns cache under given blockID
func (db *DB) getCache(blockID uint64) *memCache {
	return db.blockCache[db.consistent.FindBlock(blockID)]
}

// Get gets data for the provided key under a blockID
func (db *DB) Get(blockID uint64, key uint64) ([]byte, error) {
	// Get cache
	cache := db.getCache(blockID)
	cache.RLock()
	defer cache.RUnlock()
	// Get item from cache.
	off, ok := cache.m[key]
	if off == -1 {
		return nil, errors.New("entry deleted")
	}
	if !ok {
		return nil, nil
	}
	scratch, err := cache.data.readRaw(off, 4) // read data length
	if err != nil {
		return nil, err
	}
	dataLen := binary.LittleEndian.Uint32(scratch[:4])
	data, err := cache.data.readRaw(off, dataLen)
	if err != nil {
		return nil, err
	}
	return data[4:], nil
}

// Remove sets data offset to -1 for the key under a blockID
func (db *DB) Remove(blockID uint64, key uint64) error {
	// Get cache
	cache := db.getCache(blockID)
	cache.RLock()
	defer cache.RUnlock()
	// Get item from cache.
	if _, ok := cache.m[key]; ok {
		cache.m[key] = -1
	}
	return nil
}

// Set sets the value for the given entry for a blockID.
func (db *DB) Set(blockID uint64, key uint64, data []byte) error {
	// Get cache.
	cache := db.getCache(blockID)
	cache.Lock()
	defer cache.Unlock()
	off, err := cache.data.allocate(uint32(len(data) + 4))
	if err != nil {
		return err
	}
	var scratch [4]byte
	binary.LittleEndian.PutUint32(scratch[0:4], uint32(len(data)+4))

	if _, err := cache.data.writeAt(scratch[:], off); err != nil {
		return err
	}
	if _, err := cache.data.writeAt(data, off+4); err != nil {
		return err
	}
	cache.m[key] = off
	return nil
}

// Keys gets all keys from block cache for the provided blockID
func (db *DB) Keys(blockID uint64) []uint64 {
	// Get cache
	cache := db.getCache(blockID)
	cache.RLock()
	defer cache.RUnlock()
	// Get keys from  block cache.
	keys := make([]uint64, 0, len(cache.m))
	for k := range cache.m {
		keys = append(keys, k)
	}
	return keys
}

// Free free keeps first offset that can be free if memdb exceeds target size.
func (db *DB) Free(blockID, key uint64) error {
	// Get cache
	cache := db.getCache(blockID)
	cache.Lock()
	defer cache.Unlock()
	if cache.freeOffset > 0 {
		return nil
	}
	off, ok := cache.m[key]
	// Get item from cache.
	if ok {
		if (cache.freeOffset == 0 || cache.freeOffset < off) && float64(off) > float64(cache.data.size)*dataTableShrinkFactor {
			cache.freeOffset = off
		}
	}

	return nil
}

// Count returns the number of items in memdb.
func (db *DB) Count() uint64 {
	count := 0
	for i := 0; i < nShards; i++ {
		cache := db.blockCache[i]
		cache.RLock()
		count += len(cache.m)
		cache.RUnlock()
	}
	return uint64(count)
}

// Size returns the total size of memdb.
func (db *DB) Size() (int64, error) {
	size := int64(0)
	for i := 0; i < nShards; i++ {
		cache := db.blockCache[i]
		cache.RLock()
		size += int64(cache.data.size)
		cache.RUnlock()
	}
	return size, nil
}
