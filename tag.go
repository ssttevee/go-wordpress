package wordpress

import (
	"encoding/json"
	"fmt"
)

const CacheKeyTag = "wp_tag_%d"

// Tag represents a WordPress tag
type Tag struct {
	Term

	Link string `json:"url"`
}

// MarshalJSON marshals itself into json
func (tag *Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":   tag.Id,
		"name": tag.Name,
		"url":  tag.Link})
}

// GetTags gets all tag data from the database
func (wp *WordPress) GetTags(tagIds ...int64) ([]*Tag, error) {
	if len(tagIds) == 0 {
		return []*Tag{}, nil
	}

	var ret []*Tag
	keyMap, _ := wp.cacheGetMulti(CacheKeyTag, tagIds, &ret)

	if len(keyMap) > 0 {
		missedIds := make([]int64, 0, len(keyMap))
		for _, indices := range keyMap {
			missedIds = append(missedIds, tagIds[indices[0]])
		}

		terms, err := wp.GetTerms(missedIds...)
		if err != nil {
			return nil, err
		}

		counter := 0
		done := make(chan error)

		for _, term := range terms {
			t := Tag{Term: *term}

			t.Link = "/tag/" + t.Slug

			// insert into return set
			for _, index := range keyMap[fmt.Sprintf(CacheKeyTag, t.Id)] {
				ret[index] = &t
			}
		}

		for ; counter > 0; counter-- {
			if err := <-done; err != nil {
				return nil, err
			}
		}

		// just let this run, no callback is needed
		go wp.cacheSetByKeyMap(keyMap, ret)
	}

	return ret, nil
}
