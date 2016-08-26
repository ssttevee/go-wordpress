package wordpress

import (
	"database/sql"

	// WordPress needs mysql
	_ "github.com/go-sql-driver/mysql"
	"fmt"
)

// WordPress represents access to the WordPress database
type WordPress struct {
	db *sql.DB

	TablePrefix string

	CacheMgr   CacheManager
	FlushCache bool

	filters map[string][]interface{}
}

// New creates and returns a new WordPress connection
func New(host, user, password, database string) (*WordPress, error) {
	if password != "" {
		user += ":" + password
	}

	db, err := sql.Open("mysql", user+"@"+host+"/"+database+"?parseTime=true")
	if err != nil {
		return nil, err
	}

	return &WordPress{db: db, filters: make(map[string][]interface{})}, nil
}

// Database returns a pointer to the database connection
//
// Useful for setting max open connections or max connection lifetime
func (wp *WordPress) Database() *sql.DB {
	return wp.db
}

// Close closes the connection to the database
//
// Not very useful when the sql package is designed to have long lived connections
func (wp *WordPress) Close() error {
	return wp.db.Close()
}

func (wp *WordPress) table(table string) string {
	return wp.TablePrefix + table
}

func (wp *WordPress) AddFilter(hook string, function interface{}) {
	wp.filters[hook] = append(wp.filters[hook], function)
}

const CacheKeyOption = "wp_option_%s"
func (wp *WordPress) GetOption(name string) (value string, err error) {
	key := fmt.Sprintf(CacheKeyOption, name)
	if wp.CacheMgr != nil && !wp.FlushCache {
		if wp.cacheGet(key, &value); value != "" {
			return value, nil
		}
	}

	if err = wp.db.QueryRow(
		"SELECT option_value FROM " + wp.table("options") + " WHERE option_name = ?",
		name).Scan(&value); err != nil {
		return "", err
	}

	if wp.CacheMgr != nil {
		go wp.CacheMgr.Set(name, value)
	}

	return value, nil
}
