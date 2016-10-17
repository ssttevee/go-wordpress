package wordpress

import (
	"database/sql"

	// WordPress needs mysql
	"cloud.google.com/go/trace"
	"github.com/elgris/sqrl"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/net/context"
)

type ctxKey int

var (
	databaseKey interface{} = ctxKey(0)
	prefixKey   interface{} = ctxKey(1)
)

// WordPress represents access to the WordPress database
type WordPress struct {
	db *sql.DB

	TablePrefix string
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

	return &WordPress{db: db}, nil
}

// SetMaxOpenConns sets the max number open connections
func (wp *WordPress) SetMaxOpenConns(n int) {
	wp.db.SetMaxOpenConns(n)
}

// Close closes the connection to the database
//
// Not very useful when the sql package is designed to have long lived connections
func (wp *WordPress) Close() error {
	return wp.db.Close()
}

// NewContext returns a derived context containing the database connection
func NewContext(parent context.Context, wp *WordPress) context.Context {
	parent = context.WithValue(parent, databaseKey, wp.db)
	parent = context.WithValue(parent, prefixKey, wp.TablePrefix)

	return parent
}

func table(c context.Context, table string) string {
	prefix, ok := c.Value(prefixKey).(string)
	if !ok {
		panic("non-wordpress context")
	}

	return prefix + table
}

func database(c context.Context) *sql.DB {
	db, ok := c.Value(databaseKey).(*sql.DB)
	if !ok {
		panic("non-wordpress context")
	}

	return db
}

// GetOption returns the string value of the WordPress option
func GetOption(c context.Context, name string) (string, error) {
	span := trace.FromContext(c).NewChild("/wordpress.GetOption")
	defer span.Finish()

	span.SetLabel("wp/option/name", name)

	stmt, args, err := sqrl.Select("option_value").
		From(table(c, "options")).
		Where(sqrl.Eq{"option_name": name}).ToSql()
	if err != nil {
		return "", err
	}

	span.SetLabel("wp/query", stmt)

	var value string
	err = database(c).QueryRow(stmt, args...).Scan(&value)
	if err != nil {
		return "", err
	}

	return value, err
}
