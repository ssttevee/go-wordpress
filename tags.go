package wordpress

import (
	"encoding/json"
	"fmt"
)

// Tag represents a WordPress tag
type Tag struct {
	Term

	Link   string `json:"url"`
}

// MarshalJSON marshals itself into json
func (tag *Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id": tag.Id,
		"name": tag.Name,
		"url": tag.Link})
}

// GetTags gets all tag data from the database
func (wp *WordPress) GetTags(categoryIds ...int64) ([]*Tag, error) {
	if len(categoryIds) == 0 {
		return []*Tag{}, nil
	}

	terms, err := wp.GetTerms(categoryIds...)
	if err != nil {
		return nil, err
	}

	counter := 0
	done := make(chan error)

	ret := make([]*Tag, 0, len(categoryIds))
	for _, term := range terms {
		t := Tag{Term: *term}

		if t.Parent > 0 {
			counter++
			go func() {
				parents, err := wp.GetTags(t.Parent)
				if err != nil {
					done <- err
					return
				}

				if len(parents) == 0 {
					done <- fmt.Errorf("parent category for %d not found: %d", t.Id, t.Parent)
				}

				t.Link = parents[0].Link + "/" + t.Slug

				done <- nil
			}()
		} else {
			t.Link = "/tag/" + t.Slug
		}

		ret = append(ret ,&t)
	}

	for ; counter > 0; counter-- {
		if err := <- done; err != nil {
			return nil, err
		}
	}

	return ret, nil
}