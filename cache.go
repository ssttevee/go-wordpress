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
func (wp *WordPress) cacheGetMulti(keyFmt string, objectIds []int64, dst interface{}) (map[string][]int, error) {
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

	keys := make([]string, idsLen)
	keyMap := make(map[string][]int)

	for index, id := range objectIds {
		key := fmt.Sprintf(keyFmt, id)

		if _, ok := keyMap[key]; !ok {
			keys[index] = key
		}

		keyMap[key] = append(keyMap[key], index)
	}

	v.Set(reflect.MakeSlice(reflect.SliceOf(t), idsLen, idsLen))

	if wp.CacheMgr != nil && !wp.FlushCache {
		cacheResults := reflect.New(reflect.SliceOf(t))
		foundKeys, err := wp.CacheMgr.GetMulti(keys, cacheResults.Interface())
		if err != nil {
			return keyMap, err
		}

		cacheResults = cacheResults.Elem()
		for i, key := range foundKeys {
			for _, index := range keyMap[key] {
				v.Index(index).Set(cacheResults.Index(i))
			}

			delete(keyMap, key)
		}
	}

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

func (wp *WordPress) cacheSetByKeyMap(keyMap map[string][]int, ret interface{}) error {
	v := reflect.ValueOf(ret)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice {
		panic("ret must be an array")
	}

	keys := make([]string, 0, len(keyMap))
	src := reflect.MakeSlice(v.Type(), 0, len(keyMap))
	for key, indices := range keyMap {
		index := indices[0]
		if index >= v.Len() {
			continue
		}

		if obj := v.Index(index); !obj.IsNil() {
			keys = append(keys, key)
			src = reflect.Append(src, obj)
		}
	}

	if wp.CacheMgr != nil {
		return wp.CacheMgr.SetMulti(keys, src.Interface())
	}

	return nil
}
