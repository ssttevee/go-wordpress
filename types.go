package wordpress

import (
	"strings"
	"errors"
)

// UrlList represents a list of urls
//
// This is just a helper to split incoming space-separated values from the database
//
// Used for pinged and to-ping urls
type UrlList []string

func (list UrlList) Scan(src interface{}) error {
	if str, ok := src.([]uint8); ok {
		if len(str) > 0 {
			list = append(list, strings.Split(string(str), " ")...)
		}

		return nil
	}

	return errors.New("the source is not a string")
}