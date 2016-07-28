package wordpress

import (
	"github.com/wulijun/go-php-serialize/phpserialize"
)

type Attachment struct {
	Object

	FileName string `json:"file_name"`

	Width    int    `json:"width"`
	Height   int    `json:"height"`

	Caption  string `json:"caption"`
	AltText  string `json:"alt_text"`
}

// GetAttachments gets all attachment data from the database
func (wp *WordPress) GetAttachments(attachmentIds ...int64) ([]*Attachment, error) {
	objects, err := wp.GetObjects(attachmentIds...)
	if err != nil {
		return nil, err
	}

	ret := make([]*Attachment, 0, len(attachmentIds))
	for _, obj := range objects {
		a := &Attachment{Object: *obj}

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

					if image_meta, ok := meta["image_meta"].(map[interface{}]interface{}); ok {
						if caption, ok := image_meta["caption"].(string); ok {
							a.Caption = caption
						}

						if alt, ok := image_meta["title"].(string); ok {
							a.AltText = alt
						}
					}
				}
			}
		}

		ret = append(ret, a)
	}

	return ret, nil
}

// QueryAttachments queries the database and returns all matching attachment ids
func (wp *WordPress) QueryAttachments(q *ObjectQueryOptions) ([]int64, error) {
	q.PostType = PostTypeAttachment;

	return wp.QueryObjects(q)
}
