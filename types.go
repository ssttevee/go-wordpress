package wordpress

import (
	"errors"
	"strings"
)

// URLList represents a list of urls
//
// This is just a helper to split incoming space-separated values from the database
//
// Used for pinged and to-ping urls
type URLList []string

// Scan formats incoming data from a sql database
func (list URLList) Scan(src interface{}) error {
	if str, ok := src.([]uint8); ok {
		if len(str) > 0 {
			list = append(list, strings.Split(string(str), " ")...)
		}

		return nil
	}

	return errors.New("the source is not a string")
}
