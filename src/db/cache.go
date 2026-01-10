package db

import (
	"log"
	"sync"

	"github.com/dgraph-io/ristretto"
)

// Storing cache keys in concurrent data structurtes to allow for clearing all caches of a certain type
// If you're reading this and you know a better way to do this, please let me know!
var (
	Cache                *ristretto.Cache
	TransactionCacheKeys = struct {
		sync.RWMutex
		m map[string]struct{}
	}{m: make(map[string]struct{})}
	ItemCacheKeys = struct {
		sync.RWMutex
		m map[string]struct{}
	}{m: make(map[string]struct{})}
	AccountCacheKeys = struct {
		sync.RWMutex
		m map[string]struct{}
	}{m: make(map[string]struct{})}
)

func InitCache() {
	var err error
	Cache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 10000, // number of keys to track frequency of
		MaxCost:     10000,
		BufferItems: 64, // number of keys per Get buffer
	})
	if err != nil {
		log.Fatalf("failed to initialize cache: %v", err)
	}
}

// Transaction Cache Functions
func SetTransactionCache(cacheKey string, value interface{}) {
	TransactionCacheKeys.Lock()
	TransactionCacheKeys.m[cacheKey] = struct{}{}
	TransactionCacheKeys.Unlock()
	Cache.Set(cacheKey, value, 1)
}

func DelTransactionCache(cacheKey string) {
	TransactionCacheKeys.Lock()
	delete(TransactionCacheKeys.m, cacheKey)
	TransactionCacheKeys.Unlock()
	Cache.Del(cacheKey)
}

func ClearAllTransactionCaches() {
	TransactionCacheKeys.Lock()
	for key := range TransactionCacheKeys.m {
		Cache.Del(key)
	}
	TransactionCacheKeys.m = make(map[string]struct{})
	TransactionCacheKeys.Unlock()
}

// Item Cache Functions
func SetItemCache(cacheKey string, value interface{}) {
	ItemCacheKeys.Lock()
	ItemCacheKeys.m[cacheKey] = struct{}{}
	ItemCacheKeys.Unlock()
	Cache.Set(cacheKey, value, 1)
}

func DelItemCache(cacheKey string) {
	ItemCacheKeys.Lock()
	delete(ItemCacheKeys.m, cacheKey)
	ItemCacheKeys.Unlock()
	Cache.Del(cacheKey)
}

func ClearAllItemCaches() {
	ItemCacheKeys.Lock()
	for key := range ItemCacheKeys.m {
		Cache.Del(key)
	}
	ItemCacheKeys.m = make(map[string]struct{})
	ItemCacheKeys.Unlock()
}

// Account Cache Functions
func SetAccountCache(cacheKey string, value interface{}) {
	AccountCacheKeys.Lock()
	AccountCacheKeys.m[cacheKey] = struct{}{}
	AccountCacheKeys.Unlock()
	Cache.Set(cacheKey, value, 1)
}

func DelAccountCache(cacheKey string) {
	AccountCacheKeys.Lock()
	delete(AccountCacheKeys.m, cacheKey)
	AccountCacheKeys.Unlock()
	Cache.Del(cacheKey)
}

func ClearAllAccountCaches() {
	AccountCacheKeys.Lock()
	for key := range AccountCacheKeys.m {
		Cache.Del(key)
	}
	AccountCacheKeys.m = make(map[string]struct{})
	AccountCacheKeys.Unlock()
}
