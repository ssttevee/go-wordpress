package wordpress

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

// WordPress represents access to the WordPress database
type WordPress struct {
	db          *sql.DB

	TablePrefix string

	CacheMgr    CacheManager
	FlushCache  bool
}

// New creates and returns a new WordPress connection
func New(host, user, password, database string) (*WordPress, error) {
	if password != "" {
		user += ":" + password
	}

	db, err := sql.Open("mysql", user + "@" + host + "/" + database + "?parseTime=true")
	if err != nil {
		return nil, err;
	}

	return &WordPress{db: db}, nil
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