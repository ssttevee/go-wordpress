package wordpress

import (
	"encoding/json"
	"errors"
	"fmt"
)

const categoryCacheKey = "wp_category_%d"

type Category struct {
	Term

	Link   string `json:"url"`
}

func (cat *Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id": cat.Id,
		"parent": cat.Parent,
		"name": cat.Name,
		"url": cat.Link})
}

// GetTags gets all category data from the database
func (wp *WordPress) GetCategories(categoryIds ...int64) ([]*Category, error) {
	if len(categoryIds) == 0 {
		return []*Category{}, nil
	}

	keys := make([]string, 0, len(categoryIds))
	keyMap := make(map[string]int)

	for index, id := range categoryIds {
		key := fmt.Sprintf(categoryCacheKey, id)

		keys = append(keys, key)
		keyMap[key] = index
	}

	ret := make([]*Category, len(categoryIds))

	if !wp.FlushCache {
		cacheResults := make([]*Category, 0, len(categoryIds))
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

	missedIds := make([]int64, 0, len(keyMap))
	for _, index := range keyMap {
		missedIds = append(missedIds, categoryIds[index])
	}

	terms, err := wp.GetTerms(missedIds...)
	if err != nil {
		return nil, err
	}

	counter := 0
	done := make(chan error)

	keys = make([]string, 0, len(keyMap))
	toCache := make([]*Category, 0, len(keyMap))

	for _, term := range terms {
		c := Category{Term: *term}

		if c.Parent > 0 {
			counter++
			go func() {
				parents, err := wp.GetCategories(c.Parent)
				if err != nil {
					done <- errors.New(fmt.Sprintf("failed to get parent category for %d: %d\n%v", c.Id, c.Parent, err))
					return
				}

				if len(parents) == 0 {
					done <- errors.New(fmt.Sprintf("parent category for %d not found: %d", c.Id, c.Parent))
				}

				c.Link = parents[0].Link + "/" + c.Slug

				done <- nil
			}()
		} else {
			c.Link = "/category/" + c.Slug
		}

		ret = append(ret ,&c)

		// prepare for storing to cache
		key := fmt.Sprintf(categoryCacheKey, c.Id)

		keys = append(keys, key)
		toCache = append(toCache, &c)

		// insert into return set
		ret[keyMap[key]] = &c
	}

	for ; counter > 0; counter-- {
		if err := <- done; err != nil {
			return nil, err
		}
	}

	// just let this run, no callback is needed
	go func() {
		wp.cacheSetMulti(keys, toCache)
	}()

	return ret, nil
}