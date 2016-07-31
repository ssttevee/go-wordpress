package wordpress

import (
	"encoding/json"
	"fmt"
)

const categoryCacheKey = "wp_category_%d"

// Category represents a WordPress category
type Category struct {
	Term

	Link string `json:"url"`
}

// MarshalJSON marshals itself into json
func (cat *Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":     cat.Id,
		"parent": cat.Parent,
		"name":   cat.Name,
		"url":    cat.Link})
}

// GetCategories gets all category data from the database
func (wp *WordPress) GetCategories(categoryIds ...int64) ([]*Category, error) {
	if len(categoryIds) == 0 {
		return []*Category{}, nil
	}

	var ret []*Category
	keyMap, _ := wp.cacheGetMulti(categoryCacheKey, categoryIds, &ret)

	if len(keyMap) > 0 {
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

		keys := make([]string, 0, len(keyMap))
		toCache := make([]*Category, 0, len(keyMap))

		for _, term := range terms {
			c := Category{Term: *term}

			if c.Parent > 0 {
				counter++
				go func() {
					parents, err := wp.GetCategories(c.Parent)
					if err != nil {
						done <- fmt.Errorf("failed to get parent category for %d: %d\n%v", c.Id, c.Parent, err)
						return
					}

					if len(parents) == 0 {
						done <- fmt.Errorf("parent category for %d not found: %d", c.Id, c.Parent)
					}

					c.Link = parents[0].Link + "/" + c.Slug

					done <- nil
				}()
			} else {
				c.Link = "/category/" + c.Slug
			}

			// prepare for storing to cache
			key := fmt.Sprintf(categoryCacheKey, c.Id)

			keys = append(keys, key)
			toCache = append(toCache, &c)

			// insert into return set
			ret[keyMap[key]] = &c
		}

		for ; counter > 0; counter-- {
			if err := <-done; err != nil {
				return nil, err
			}
		}

		// just let this run, no callback is needed
		go func() {
			_ = wp.cacheSetMulti(keys, toCache)
		}()
	}

	return ret, nil
}
