package wordpress

import (
	"errors"
	"fmt"
	"reflect"
)

// ErrMissedCache is returned when a key is not found in the cache
var ErrMissedCache = errors.New("couldn't find item in cache")

// CacheManager is used for managing cache
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
	SetMulti(key []string, src interface{}) error
}

func (wp *WordPress) cacheGet(key string, dst interface{}) error {
	if wp.CacheMgr != nil {
		return wp.CacheMgr.Get(key, dst)
	}

	return ErrMissedCache
}

// returns keyMap
func (wp *WordPress) cacheGetMulti(keyFmt string, objectIds []int64, dst interface{}) (map[string]int, error) {
	v := reflect.ValueOf(dst)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if !v.CanAddr() {
		panic("dst must be a pointer")
	}

	if v.Kind() != reflect.Slice {
		panic("dst must point to a slice")
	}

	t := v.Type().Elem()

	idsLen := len(objectIds)

	keys := make([]string, 0, idsLen)
	keyMap := make(map[string]int)

	for index, id := range objectIds {
		key := fmt.Sprintf(keyFmt, id)

		keys = append(keys, key)
		keyMap[key] = index
	}

	ret := reflect.MakeSlice(reflect.SliceOf(t), idsLen, idsLen)

	if wp.CacheMgr != nil && !wp.FlushCache {
		cacheResults := reflect.New(reflect.SliceOf(t))
		keys, err := wp.CacheMgr.GetMulti(keys, cacheResults.Interface())
		if err != nil {
			return keyMap, err
		}

		cacheResults = cacheResults.Elem()
		for i, key := range keys {
			ret.Index(keyMap[key]).Set(cacheResults.Index(i))

			delete(keyMap, key)
		}
	}

	v.Set(ret)

	return keyMap, nil
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
