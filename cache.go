package wordpress

import (
	"errors"
)

var ErrMissedCache = errors.New("couldn't find item in cache")

// Interface for managing cache
type CacheManager interface {
	// Get retrieves single item from cache and stores it into dst
	//
	// If they key is missed, ErrMissedCache error is returned
	Get(key string, dst interface{}) error
	// GetMulti retrieves several items from the cache
	// into dst and returns the keys in the retrieved order
	GetMulti(keys []string, dst interface{}) ([]string, error)

	// Set stores a single item in the cache
	//
	// There is no indication of whether the item actually gets stored
	Set(key string, src interface{}) error
	// SetMulti stores several items in the cache
	//
	// There is no indication of whether they all get stored
	SetMulti(key[] string, src interface{}) error
}

func (wp *WordPress) cacheGet(key string, dst interface{}) error {
	if wp.CacheMgr != nil {
		return wp.CacheMgr.Get(key, dst)
	}

	return ErrMissedCache
}

func (wp *WordPress) cacheGetMulti(keys []string, dst interface{}) ([]string, error) {
	if wp.CacheMgr != nil {
		return wp.CacheMgr.GetMulti(keys, dst)
	}

	return []string{}, nil
}

func (wp *WordPress) cacheSet(key string, dst interface{}) error {
	if wp.CacheMgr != nil {
		return wp.CacheMgr.Set(key, dst)
	}

	return nil
}

func (wp *WordPress) cacheSetMulti(keys []string, dst interface{}) error {
	if wp.CacheMgr != nil {
		return wp.CacheMgr.SetMulti(keys, dst)
	}

	return nil
}