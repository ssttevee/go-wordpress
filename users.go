package wordpress

import (
	"time"
	"crypto/md5"
	"strings"
	"fmt"
)

const userCacheKey = "wp_user_%d"

type User struct {
	Id          int64     `json:"id"`
	Slug        string    `json:"slug"`

	Name        string    `json:"name"`
	Description string    `json:"description"`

	Email       string    `json:"-"` // don't leak my email info!! >:[
	Gravatar    string    `json:"gravatar"`
	Website     string    `json:"url"`

	Registered  time.Time `json:"-"`
}

// GetObjects gets all user data from the database
func (wp *WordPress) GetUsers(userIds ...int64) ([]*User, error) {
	if len(userIds) == 0 {
		return []*User{}, nil
	}

	keys := make([]string, 0, len(userIds))
	keyMap := make(map[string]int)

	for index, id := range userIds {
		key := fmt.Sprintf(userCacheKey, id)

		keys = append(keys, key)
		keyMap[key] = index
	}

	ret := make([]*User, len(userIds))

	if !wp.FlushCache {
		cacheResults := make([]*User, 0, len(userIds))
		if keys, err := wp.cacheGetMulti(keys, &cacheResults); err == nil {
			for i, key := range keys {
				ret[keyMap[key]] = cacheResults[i]

				delete(keyMap, key)
			}
		}

		if len(keyMap) == 0 {
			return ret, nil
		}
	}


	params := make([]interface{}, 0)
	stmt := "SELECT u.ID, u.user_nicename, u.display_name, um.meta_value, u.user_email, u.user_url, u.user_registered " +
		"FROM " + wp.table("users") + " AS u JOIN " + wp.table("usermeta") + " AS um ON um.user_id = u.ID " +
		"WHERE um.meta_key = ? AND u.ID IN ("
	params = append(params, "description")
	for _, index := range keyMap {
		stmt += "?,"
		params = append(params, userIds[index])
	}
	stmt = stmt[:len(stmt) - 1] + ") GROUP BY u.ID"

	rows, err := wp.db.Query(stmt, params...)
	if err != nil {
		return nil, err
	}

	keys = make([]string, 0, len(keyMap))
	toCache := make([]*User, 0, len(keyMap))

	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Id, &u.Slug, &u.Name, &u.Description, &u.Email, &u.Website, &u.Registered); err != nil {
			return nil, err
		}

		u.Gravatar = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(u.Email)))))

		// prepare for storing to cache
		key := fmt.Sprintf(userCacheKey, u.Id)

		keys = append(keys, key)
		toCache = append(toCache, &u)

		// insert into return set
		ret[keyMap[key]] = &u
	}

	// just let this run, no callback is needed
	go func() {
		wp.cacheSetMulti(keys, toCache)
	}()

	return ret, nil
}
