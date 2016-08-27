package wordpress

import (
	"crypto/md5"
	"fmt"
	"strings"
	"time"
)

const CacheKeyUser = "wp_user_%d"

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

// GetUsers gets all user data from the database
func (wp *WordPress) GetUsers(userIds ...int64) ([]*User, error) {
	if len(userIds) == 0 {
		return []*User{}, nil
	}

	var ret []*User
	keyMap, _ := wp.cacheGetMulti(CacheKeyUser, userIds, &ret)

	if len(keyMap) > 0 {
		var params []interface{}
		stmt := "SELECT u.ID, u.user_nicename, u.display_name, um.meta_value, u.user_email, u.user_url, u.user_registered " +
			"FROM " + wp.table("users") + " AS u JOIN " + wp.table("usermeta") + " AS um ON um.user_id = u.ID " +
			"WHERE um.meta_key = ? AND u.ID IN ("
		params = append(params, "description")
		for _, indices := range keyMap {
			stmt += "?,"
			params = append(params, userIds[indices[0]])
		}
		stmt = stmt[:len(stmt)-1] + ") GROUP BY u.ID"

		rows, err := wp.db.Query(stmt, params...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var u User
			if err := rows.Scan(&u.Id, &u.Slug, &u.Name, &u.Description, &u.Email, &u.Website, &u.Registered); err != nil {
				return nil, err
			}

			u.Gravatar = fmt.Sprintf("%x", md5.Sum([]byte(strings.ToLower(strings.TrimSpace(u.Email)))))

			// insert into return set
			for _, index := range keyMap[fmt.Sprintf(CacheKeyUser, u.Id)] {
				ret[index] = &u
			}
		}

		// just let this run, no callback is needed
		go wp.cacheSetByKeyMap(keyMap, ret)
	}

	return ret, nil
}
