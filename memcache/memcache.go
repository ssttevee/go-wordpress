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

type Memcache struct {
	c context.Context

	Expiration time.Duration
}

func New(c context.Context) *Memcache {
	return &Memcache{c, time.Minute * 15}
}

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

func (m *Memcache) GetMulti(keys []string, dst interface{}) ([]string, error) {
	v := reflect.ValueOf(dst)

	if v.Kind() != reflect.Ptr {
		return nil, errors.New("wp memcache: dst must be a pointer")
	}

	v = reflect.ValueOf(dst).Elem()

	if v.Kind() != reflect.Slice {
		return nil, errors.New("wp memcache: dst must be a pointer to a slice")
	}

	t := v.Type().Elem()

	items, err := memcache.GetMulti(m.c, keys)
	if err != nil {
		return nil, err
	}

	keyMap := make(map[string]int)
	for i, key := range keys {
		keyMap[key] = i
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
		delete(keyMap, item.Key)
	}

	return retKeys, nil
}

func (m *Memcache) Set(key string, src interface{}) error {
	item := memcache.Item{Key: key}

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(src); err != nil {
		return err
	}

	item.Value = buf.Bytes()
	item.Expiration = m.Expiration

	return memcache.Set(m.c, &item)
}

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
		item.Expiration = m.Expiration

		items = append(items, &item)
	}

	return memcache.SetMulti(m.c, items)
}
