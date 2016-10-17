package wordpress

import (
	"errors"
)

type Iterator interface {
	Next() (int64, error)
	Cursor() string
	Slice() ([]int64, error)
}

var (
	Done     = errors.New("wordpress: no more rows to read")
	zeroIter = &iteratorImpl{next: func() (int64, error) {
		return 0, Done
	}}
)

type iteratorImpl struct {
	next   func() (int64, error)
	cursor string
}

func (it *iteratorImpl) Next() (int64, error) {
	return it.next()
}

func (it *iteratorImpl) Cursor() string {
	return it.cursor
}

func (it *iteratorImpl) Slice() (ret []int64, err error) {
	for {
		var id int64
		if id, err = it.Next(); err == Done {
			break
		} else if err != nil {
			return nil, err
		}

		ret = append(ret, id)
	}

	return ret, nil
}

func (it *iteratorImpl) exit(err error) (int64, error) {
	it.next = func() (int64, error) {
		return 0, err
	}

	return it.next()
}
