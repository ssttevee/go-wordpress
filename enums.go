package wordpress

import "errors"

// Posts in WordPress can have one of a number of statuses.
//
// The status of a given post determines how WordPress handles that post.
type PostStatus string

const (
	PostStatusPublish   PostStatus = "publish"
	PostStatusFuture    PostStatus = "future"
	PostStatusDraft     PostStatus = "draft"
	PostStatusPending   PostStatus = "pending"
	PostStatusPrivate   PostStatus = "private"
	PostStatusTrash     PostStatus = "trash"
	PostStatusAutoDraft PostStatus = "auto-draft"
	PostStatusInherit   PostStatus = "inherit"
)

func (s PostStatus) Scan(src interface{}) error {
	if arr, ok := src.([]uint8); !ok {
		return errors.New("the source is not a []uint8")
	} else {
		s = PostStatus(arr)
		return nil
	}
}

// There are five post types that are readily available to users
// or internally used by the WordPress installation by default:
//
// Post, Page, Attachment, Revision, Navigation Menu Item
//
// https://codex.wordpress.org/Post_Types
type PostType string

const (
	PostTypeAttachment  PostType = "attachment"
	PostTypeNavMenuItem PostType = "nav_menu_item"
	PostTypePage        PostType = "page"
	PostTypePost        PostType = "post"
	PostTypeRevision    PostType = "revision"
)

func (t PostType) Scan(src interface{}) error {
	if arr, ok := src.([]uint8); !ok {
		return errors.New("the source is not a []uint8")
	} else {
		t = PostType(arr)
		return nil
	}
}

// MenuItemType represents menu item link types
//
// i.e. post_type, taxonomy, custom
type MenuItemType string

const (
	MenuItemTypePost     MenuItemType = "post_type"
	MenuItemTypeTaxonomy MenuItemType = "taxonomy"
	MenuItemTypeCustom   MenuItemType = "custom"
)

// Taxonomy represents term taxonomy names
//
// i.e. categories, nav_menu, post_tag
type Taxonomy string

const (
	TaxonomyCategory Taxonomy = "category"
	TaxonomyNavMenu  Taxonomy = "nav_menu"
	TaxonomyPostTag  Taxonomy = "post_tag"
)