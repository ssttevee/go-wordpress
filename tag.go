package wordpress

import (
	"cloud.google.com/go/trace"
	"encoding/json"
	"golang.org/x/net/context"
)

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
func GetTags(c context.Context, tagIds ...int64) ([]*Tag, error) {
	span := trace.FromContext(c).NewChild("/wordpress.GetTags")
	defer span.Finish()

	if len(tagIds) == 0 {
		return nil, nil
	}

	ids, idMap := dedupe(tagIds)

	ret := make([]*Tag, len(tagIds))
	terms, err := getTerms(c, ids...)
	if err != nil {
		return nil, err
	}

	for _, term := range terms {
		t := Tag{Term: *term}

		t.Link = "/tag/" + t.Slug

		// insert into return set
		for _, index := range idMap[t.Id] {
			ret[index] = &t
		}
	}

	return ret, nil
}
