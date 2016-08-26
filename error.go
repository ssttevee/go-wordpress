package wordpress

import (
	"bytes"
	"strconv"
)

type MissingResourcesError []int64

func (ids MissingResourcesError) Error() string {
	var msg bytes.Buffer

	msg.WriteString("wordpress: could not find ids ")
	for _, id := range ids[:len(ids)-1] {
		msg.WriteString(strconv.FormatInt(id, 10))
		msg.WriteRune(',')
		msg.WriteRune(' ')
	}
	msg.Truncate(msg.Len()-2)
	if len(ids) > 1 {
		msg.WriteString(" and")
	}
	msg.WriteRune(' ')
	msg.WriteString(strconv.FormatInt(ids[len(ids)-1], 10))

	return msg.String()
}