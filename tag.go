package wordpress

import (
	"go.opencensus.io/trace"
	"encoding/json"
	"golang.org/x/net/context"
	"strings"
	"errors"
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
	c, span := trace.StartSpan(c, "/wordpress.GetTags")
	defer span.End()

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

// GetTagIdBySlug returns the id of the category that matches the given slug
func GetTagIdBySlug(c context.Context, slug string) (int64, error) {
	c, span := trace.StartSpan(c, "/wordpress.GetTagIdBySlug")
	span.End()

	parts := strings.Split(slug, "/")

	var tagId int64
	for _, part := range parts {
		it, err := queryTerms(c, &TermQueryOptions{
			Taxonomy: TaxonomyPostTag,
			Slug:     part,
			ParentId: tagId})
		if err != nil {
			return 0, err
		}

		if tagId, err = it.Next(); err != nil {
			return 0, errors.New("wordpress: non-existent tag slug")
		}
	}

	return tagId, nil
}
