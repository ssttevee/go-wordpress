package wordpress

import (
	"fmt"
	"strconv"
)

const CacheKeyPost = "wp_post_%d"

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

const FilterAfterGetPosts = "after_get_posts"
type FilterAfterGetPostsFunc func(*WordPress, []*Post) ([]*Post, error)

// GetPosts gets all post data from the database
func (wp *WordPress) GetPosts(postIds ...int64) ([]*Post, error) {
	if len(postIds) == 0 {
		return []*Post{}, nil
	}

	var ret []*Post
	keyMap, _ := wp.cacheGetMulti(CacheKeyPost, postIds, &ret)

	if len(keyMap) > 0 {
		missedIds := make([]int64, 0, len(keyMap))
		for _, indices := range keyMap {
			missedIds = append(missedIds, postIds[indices[0]])
		}

		objects, err := wp.GetObjects(missedIds...)
		if err != nil {
			return nil, err
		}

		counter := 0
		done := make(chan error)

		for _, obj := range objects {
			p := Post{Object: *obj}

			counter++
			go func() {
				if meta, err := p.GetMeta(); err != nil {
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

			// insert into return set
			for _, index := range keyMap[fmt.Sprintf(CacheKeyPost, p.Id)] {
				ret[index] = &p
			}
		}

		for ; counter > 0; counter-- {
			if err := <-done; err != nil {
				return nil, err
			}
		}

		for _, filter := range wp.filters[FilterAfterGetPosts] {
			f, ok := filter.(FilterAfterGetPostsFunc)
			if !ok {
				panic("got a bad filter for '" + FilterAfterGetPosts + "'")
			}

			ret, err = f(wp, ret)
			if err != nil {
				return nil, err
			}
		}

		// just let this run, no callback is needed
		go wp.cacheSetByKeyMap(keyMap, ret)
	}

	return ret, nil
}

// QueryPosts queries the database and returns all matching prost ids
func (wp *WordPress) QueryPosts(q *ObjectQueryOptions) ([]int64, error) {
	q.PostStatus = PostStatusPublish

	if q.PostType == "" {
		q.PostType = PostTypePost
	}

	return wp.QueryObjects(q)
}
