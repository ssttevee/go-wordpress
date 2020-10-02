package wordpress

import (
	"go.opencensus.io/trace"
	"github.com/wulijun/go-php-serialize/phpserialize"
	"golang.org/x/net/context"
)

// Attachment represents a WordPress attachment
type Attachment struct {
	Object

	FileName string `json:"file_name"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	Caption string `json:"caption"`
	AltText string `json:"alt_text"`

	Url string `json:"url,omitempty"`
}

// GetAttachments gets all attachment data from the database
func GetAttachments(c context.Context, attachmentIds ...int64) ([]*Attachment, error) {
	c, span := trace.StartSpan(c, "/wordpress.GetAttachments")
	defer span.End()

	if len(attachmentIds) == 0 {
		return nil, nil
	}

	ids, idMap := dedupe(attachmentIds)

	objects, err := getObjects(c, ids...)
	if err != nil {
		return nil, err
	}

	baseUrl, _ := GetOption(c, "upload_url_path")
	if baseUrl == "" {
		siteUrl, _ := GetOption(c, "siteurl")
		baseDir, _ := GetOption(c, "upload_path")
		if baseDir == "" {
			baseDir = "/wp-content/uploads"
		}

		baseUrl = siteUrl + baseDir
	}

	ret := make([]*Attachment, len(attachmentIds))
	for _, obj := range objects {
		att := Attachment{Object: *obj}

		meta, err := att.GetMeta(c, "_wp_attachment_metadata")
		if err != nil {
			return nil, err
		}

		if enc, ok := meta["_wp_attachment_metadata"]; ok && enc != "" {
			if dec, err := phpserialize.Decode(enc); err == nil {
				if meta, ok := dec.(map[interface{}]interface{}); ok {
					if file, ok := meta["file"].(string); ok {
						att.FileName = file
					}

					if width, ok := meta["width"].(int64); ok {
						att.Width = int(width)
					}

					if height, ok := meta["height"].(int64); ok {
						att.Height = int(height)
					}

					if imageMeta, ok := meta["image_meta"].(map[interface{}]interface{}); ok {
						if caption, ok := imageMeta["caption"].(string); ok {
							att.Caption = caption
						}

						if alt, ok := imageMeta["title"].(string); ok {
							att.AltText = alt
						}
					}
				}
			}
		}

		att.Url = baseUrl + att.Date.Format("/2006/01/") + att.FileName

		// insert into return set
		for _, index := range idMap[att.Id] {
			ret[index] = &att
		}
	}

	return ret, nil
}

// QueryAttachments returns the ids of the attachments that match the query
func QueryAttachments(c context.Context, opts *ObjectQueryOptions) (Iterator, error) {
	c, span := trace.StartSpan(c, "/wordpress.QueryAttachments")
	defer span.End()

	opts.PostType = PostTypeAttachment

	return queryObjects(c, opts)
}
