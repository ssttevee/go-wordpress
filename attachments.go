package wordpress

import (
	"fmt"
	"github.com/wulijun/go-php-serialize/phpserialize"
)

const CacheKeyAttachment = "wp_attachment_%d"

// Attachment represents a WordPress attachment
type Attachment struct {
	Object

	FileName string `json:"file_name"`

	Width  int `json:"width"`
	Height int `json:"height"`

	Caption string `json:"caption"`
	AltText string `json:"alt_text"`
}

// GetAttachments gets all attachment data from the database
func (wp *WordPress) GetAttachments(attachmentIds ...int64) ([]*Attachment, error) {
	objects, err := wp.GetObjects(attachmentIds...)
	if err != nil {
		return nil, err
	}

	var ret []*Attachment
	keyMap, _ := wp.cacheGetMulti(CacheKeyAttachment, attachmentIds, &ret)

	if len(keyMap) > 0 {
		missedIds := make([]int64, 0, len(keyMap))
		for _, index := range keyMap {
			missedIds = append(missedIds, attachmentIds[index])
		}

		keys := make([]string, 0, len(keyMap))
		toCache := make([]*Attachment, 0, len(keyMap))

		for _, obj := range objects {
			a := Attachment{Object: *obj}

			meta, err := a.GetMeta("_wp_attachment_metadata")
			if err != nil {
				return nil, err
			}

			if enc, ok := meta["_wp_attachment_metadata"]; ok && enc != "" {
				if dec, err := phpserialize.Decode(enc); err == nil {
					if meta, ok := dec.(map[interface{}]interface{}); ok {
						if file, ok := meta["file"].(string); ok {
							a.FileName = file
						}

						if width, ok := meta["width"].(int64); ok {
							a.Width = int(width)
						}

						if height, ok := meta["height"].(int64); ok {
							a.Height = int(height)
						}

						if imageMeta, ok := meta["image_meta"].(map[interface{}]interface{}); ok {
							if caption, ok := imageMeta["caption"].(string); ok {
								a.Caption = caption
							}

							if alt, ok := imageMeta["title"].(string); ok {
								a.AltText = alt
							}
						}
					}
				}
			}

			// prepare for storing to cache
			key := fmt.Sprintf(CacheKeyAttachment, a.Id)

			keys = append(keys, key)
			toCache = append(toCache, &a)

			// insert into return set
			ret[keyMap[key]] = &a
		}

		// just let this run, no callback is needed
		go func() {
			_ = wp.cacheSetMulti(keys, toCache)
		}()
	}

	return ret, nil
}

// QueryAttachments queries the database and returns all matching attachment ids
func (wp *WordPress) QueryAttachments(q *ObjectQueryOptions) ([]int64, error) {
	q.PostType = PostTypeAttachment

	return wp.QueryObjects(q)
}
