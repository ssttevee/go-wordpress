package memcache

import (
	"bytes"
	"encoding/gob"
	"errors"
	"github.com/ssttevee/go-wordpress"
	"golang.org/x/net/context"
	"google.golang.org/appengine/memcache"
	"reflect"
	"time"
)

// Expiration is the how long things will be stored in the cache
var Expiration = time.Minute * 15

// Memcache is an adapter for using the app engine memcache with the wordpress package
type Memcache struct {
	c context.Context
}

// New creates a new memcache adapter
func New(c context.Context) *Memcache {
	return &Memcache{c: c}
}

// Get retrieves single item from cache and stores it into dst
//
// If they key is missed, ErrMissedCache error is returned
func (m *Memcache) Get(key string, dst interface{}) error {
	item, err := memcache.Get(m.c, key)
	if err == memcache.ErrCacheMiss {
		return wordpress.ErrMissedCache
	} else if err != nil {
		return err
	}

	dec := gob.NewDecoder(bytes.NewReader(item.Value))

	if err := dec.Decode(dst); err != nil {
		return err
	}

	return nil
}

// GetMulti retrieves several items from the cache
// into dst and returns the keys in the retrieved order
func (m *Memcache) GetMulti(keys []string, dst interface{}) ([]string, error) {
	v := reflect.ValueOf(dst)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if !v.CanAddr() {
		panic("wp memcache: dst must be a pointer")
	}

	if v.Kind() != reflect.Slice {
		panic("wp memcache: dst must point to a slice")
	}

	t := v.Type().Elem()

	items, err := memcache.GetMulti(m.c, keys)
	if err != nil {
		return nil, err
	}

	retKeys := make([]string, 0, len(items))
	for _, item := range items {
		var nv reflect.Value
		if t.Kind() == reflect.Ptr {
			nv = reflect.New(t.Elem())
		} else {
			nv = reflect.New(t)
		}

		dec := gob.NewDecoder(bytes.NewReader(item.Value))
		if err := dec.DecodeValue(nv); err != nil {
			return nil, err
		}

		if t.Kind() != reflect.Ptr {
			nv = nv.Elem()
		}

		retKeys = append(retKeys, item.Key)
		v.Set(reflect.Append(v, nv))
	}

	return retKeys, nil
}

// Set stores a single item in the cache
//
// There is no indication of whether the item actually gets stored
func (m *Memcache) Set(key string, src interface{}) error {
	item := memcache.Item{Key: key}

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(src); err != nil {
		return err
	}

	item.Value = buf.Bytes()
	item.Expiration = Expiration

	return memcache.Set(m.c, &item)
}

// SetMulti stores several items in the cache
//
// There is no indication of whether they all get stored
func (m *Memcache) SetMulti(keys []string, src interface{}) error {
	v := reflect.ValueOf(src)

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice {
		return errors.New("wp memcache: src must be a slice")
	}

	if len(keys) != v.Len() {
		return errors.New("wp memcache: keys and src do not match in length")
	}

	items := make([]*memcache.Item, 0, len(keys))
	for i := 0; i < v.Len(); i++ {
		v := v.Index(i)

		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		item := memcache.Item{Key: keys[i]}

		buf := bytes.Buffer{}
		enc := gob.NewEncoder(&buf)
		if err := enc.EncodeValue(v); err != nil {
			return err
		}

		item.Value = buf.Bytes()
		item.Expiration = Expiration

		items = append(items, &item)
	}

	return memcache.SetMulti(m.c, items)
}
