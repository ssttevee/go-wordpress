package wordpress

import (
	"strconv"
)

type Post struct {
	Object

	// The post's featured_media
	FeaturedMediaId int64             `json:"featured_media,omitempty"`

	// The post's categories
	CategoryIds     []int64           `json:"categories"`

	// The post's tags
	TagIds          []int64           `json:"tags"`

	// The post's metadata
	Meta            map[string]string `json:"meta"`
}

// GetTags gets all post data from the database
func (wp *WordPress) GetPosts(postIds ...int64) ([]*Post, error) {
	if len(postIds) == 0 {
		return []*Post{}, nil
	}

	objects, err := wp.GetObjects(postIds...)
	if err != nil {
		return nil, err
	}

	counter := 0
	done := make(chan error)

	ret := make([]*Post, 0, len(postIds))
	for _, obj := range objects {
		p := &Post{Object: *obj}

		counter++
		go func() {
			if meta, err := p.GetMeta(); err != nil {
				done <- err
			} else {
				if thumbnailId, ok := meta["_thumbnail_id"]; ok {
					p.FeaturedMediaId, _ = strconv.ParseInt(thumbnailId, 10, 64)
					delete(meta, "_thumbnail_id")
				}

				p.Meta = meta

				done <- nil
			}
		}()

		counter++
		go func() {
			if ids, err := p.GetTaxonomy(TaxonomyCategory); err != nil {
				done <- err
			} else {
				p.CategoryIds = ids
				done <- nil
			}
		}()

		counter++
		go func() {
			if ids, err := p.GetTaxonomy(TaxonomyPostTag); err != nil {
				done <- err
			} else {
				p.TagIds = ids
				done <- nil
			}
		}()

		ret = append(ret, p)
	}

	for ; counter > 0; counter-- {
		if err := <- done; err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (wp *WordPress) QueryPosts(q *ObjectQueryOptions) ([]int64, error) {
	q.PostStatus = PostStatusPublish

	if q.PostType == "" {
		q.PostType = PostTypePost
	}

	return wp.QueryObjects(q)
}