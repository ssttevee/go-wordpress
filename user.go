package wordpress

import (
	"cloud.google.com/go/trace"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"github.com/elgris/sqrl"
	"golang.org/x/net/context"
	"strconv"
	"strings"
	"time"
)

// User represents a WordPress user
type User struct {
	Id   int64  `json:"id"`
	Slug string `json:"slug"`

	Name        string `json:"name"`
	Description string `json:"description"`

	Email    string `json:"-"` // don't leak my email info!! >:[
	Gravatar string `json:"gravatar"`
	Website  string `json:"url"`

	Registered time.Time `json:"-"`
}

// UserQueryOptions represents the available parameters for querying
type UserQueryOptions struct {
	After string `param:"after"`
	Limit int    `param:"limit"`

	Id      int64   `param:"user_id"`
	IdIn    []int64 `param:"user_id__in"`
	IdNotIn []int64 `param:"user_id__not_in"`

	Slug      string   `param:"slug"`
	SlugIn    []string `param:"slug__in"`
	SlugNotIn []string `param:"slug__not_in"`
}

// GetUsers gets all user data from the database
func GetUsers(c context.Context, userIds ...int64) ([]*User, error) {
	span := trace.FromContext(c).NewChild("/wordpress.GetUsers")
	defer span.Finish()

	if len(userIds) == 0 {
		return []*User{}, nil
	}

	ids, idMap := dedupe(userIds)

	stmt, args, err := sqrl.Select("u.ID", "u.user_nicename", "u.display_name", "um.meta_value", "u.user_email", "u.user_url", "u.user_registered").
		From(table(c, "users") + " AS u").
		Join(table(c, "usermeta") + " AS um ON um.user_id = u.ID").
		Where(sqrl.Eq{"meta_key": "description"}).
		GroupBy("u.ID").
		Where(sqrl.Eq{"u.ID": ids}).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := database(c).Query(stmt, args...)
	if err != nil {
		return nil, err
	}

	ret := make([]*User, len(userIds))
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Id, &u.Slug, &u.Name, &u.Description, &u.Email, &u.Website, &u.Registered); err != nil {
			return nil, err
		}

		u.Gravatar = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(u.Email)))))

		// insert into return set
		for _, index := range idMap[u.Id] {
			ret[index] = &u
		}
	}

	var mre MissingResourcesError
	for i, term := range ret {
		if term == nil {
			mre = append(mre, userIds[i])
		}
	}

	if len(mre) > 0 {
		return nil, err
	}

	return ret, nil
}

// QueryUsers returns the ids of the users that match the query
func QueryUsers(c context.Context, opts *UserQueryOptions) (Iterator, error) {
	span := trace.FromContext(c).NewChild("/wordpress.QueryUsers")
	defer span.Finish()

	q := sqrl.Select("ID").From(table(c, "users")).OrderBy("ID ASC")

	if opts.Id != 0 {
		q = q.Where(sqrl.Eq{"ID": opts.Id})
	} else if len(opts.IdIn) > 0 {
		q = q.Where(sqrl.Eq{"ID": opts.IdIn})
	} else if len(opts.IdNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"ID": opts.IdNotIn})
	}

	if opts.Slug != "" {
		q = q.Where(sqrl.Eq{"user_nicename": opts.Slug})
	} else if len(opts.SlugIn) > 0 {
		q = q.Where(sqrl.Eq{"user_nicename": opts.SlugIn})
	} else if len(opts.SlugNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"user_nicename": opts.SlugNotIn})
	}

	if opts.After != "" {
		// ignore `q.After` if any errors occur
		if b, err := base64.URLEncoding.DecodeString(opts.After); err == nil {
			q = q.Where("ID > ?", string(b))
		}
	}

	if opts.Limit == 0 {
		opts.Limit = 10
	}

	if opts.Limit > 0 {
		q = q.Limit(uint64(opts.Limit))
	}

	stmt, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	span.SetLabel("wp/user/query", stmt)

	rows, err := database(c).Query(stmt, args...)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	span.SetLabel("wp/user/count", strconv.Itoa(len(ids)))

	it := iteratorImpl{cursor: opts.After}

	var counter int
	it.next = func() (id int64, err error) {
		if counter < len(ids) {
			id = ids[counter]
			it.cursor = base64.URLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
			counter++
		} else {
			return it.exit(Done)
		}

		return id, err
	}

	return &it, nil
}
