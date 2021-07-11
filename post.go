package wordpress

import (
	"go.opencensus.io/trace"
	"golang.org/x/net/context"
	"strconv"
)

// Post represents a WordPress post
type Post struct {
	Object

	// The post's featured_media
	FeaturedMediaId int64 `json:"featured_media,omitempty"`

	// The post's categories
	CategoryIds []int64 `json:"categories"`

	// The post's tags
	TagIds []int64 `json:"tags"`

	// The post's metadata
	Meta map[string]string `json:"meta"`
}

// GetPosts gets all post data from the database
func GetPosts(c context.Context, postIds ...int64) ([]*Post, error) {
	c, span := trace.StartSpan(c, "/wordpress.GetPosts")
	defer span.End()

	if len(postIds) == 0 {
		return nil, nil
	}

	ids, idMap := dedupe(postIds)

	objects, err := getObjects(c, ids...)
	if err != nil {
		return nil, err
	}

	counter := 0
	done := make(chan error)

	ret := make([]*Post, len(postIds))
	for _, obj := range objects {
		p := Post{Object: *obj}

		counter++
		go func() {
			if meta, err := p.GetMeta(c); err != nil {
				done <- err
			} else {
				if thumbnailId, ok := meta["_thumbnail_id"]; ok {
					p.FeaturedMediaId, _ = strconv.ParseInt(thumbnailId, 10, 64)
					delete(meta, "_thumbnail_id")
				}

				// clear the internal use metadata
				for metaKey := range meta {
					if metaKey[0] == '_' {
						delete(meta, metaKey)
					}
				}

				p.Meta = meta

				done <- nil
			}
		}()

		counter++
		go func() {
			if it, err := p.GetTaxonomy(c, TaxonomyCategory); err != nil {
				done <- err
			} else {
				p.CategoryIds, err = it.Slice()
				done <- err
			}
		}()

		counter++
		go func() {
			if it, err := p.GetTaxonomy(c, TaxonomyPostTag); err != nil {
				done <- err
			} else {
				p.TagIds, err = it.Slice()
				done <- err
			}
		}()

		// insert into return set
		for _, index := range idMap[p.Id] {
			ret[index] = &p
		}
	}

	for ; counter > 0; counter-- {
		if err := <-done; err != nil {
			return nil, err
		}
	}

	return ret, nil
}

// QueryPosts returns the ids of the posts that match the query
func QueryPosts(c context.Context, opts *ObjectQueryOptions) (Iterator, error) {
	c, span := trace.StartSpan(c, "/wordpress.QueryPosts")
	defer span.End()

	if opts.PostStatus == "" {
		opts.PostStatus = PostStatusPublish
	}

	if opts.PostType == "" {
		opts.PostType = PostTypePost
	}

	return queryObjects(c, opts)
}
