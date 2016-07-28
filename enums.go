package wordpress

import "errors"

// PostStatus represents a WordPress post status
//
// Posts in WordPress can have one of a number of statuses.
//
// The status of a given post determines how WordPress handles that post.
type PostStatus string

const (
	// PostStatusPublish is a published post
	PostStatusPublish   PostStatus = "publish"

	// PostStatusFuture is a post which has been
	// published but it's publish date is in the future
	PostStatusFuture    PostStatus = "future"

	// PostStatusDraft is a draft post
	PostStatusDraft     PostStatus = "draft"

	// PostStatusPending is a post which is awaiting approval
	PostStatusPending   PostStatus = "pending"

	// PostStatusPrivate is a private post
	PostStatusPrivate   PostStatus = "private"

	// PostStatusTrash is a post that was trashed
	PostStatusTrash     PostStatus = "trash"

	// PostStatusAutoDraft is an auto-saved post
	PostStatusAutoDraft PostStatus = "auto-draft"

	// PostStatusInherit inherits its status from its parent
	PostStatusInherit   PostStatus = "inherit"
)

// Scan formats incoming data from a sql database
func (s PostStatus) Scan(src interface{}) error {
	if arr, ok := src.([]uint8); ok {
		s = PostStatus(arr)
		return nil
	}

	return errors.New("the source is not a []uint8")
}

// PostType represents a WordPress post type
//
// There are five post types that are readily available to users
// or internally used by the WordPress installation by default:
//
// Post, Page, Attachment, Revision, Navigation Menu Item
//
// https://codex.wordpress.org/Post_Types
type PostType string

const (
	// PostTypeAttachment is an attachment
	PostTypeAttachment  PostType = "attachment"

	// PostTypeNavMenuItem is a menu item
	PostTypeNavMenuItem PostType = "nav_menu_item"

	// PostTypePage is a page
	PostTypePage        PostType = "page"

	// PostTypePost is a post
	PostTypePost        PostType = "post"

	// PostTypeRevision is a revision
	PostTypeRevision    PostType = "revision"
)

// Scan formats incoming data from a sql database
func (t PostType) Scan(src interface{}) error {
	if arr, ok := src.([]uint8); ok {
		t = PostType(arr)
		return nil
	}

	return errors.New("the source is not a []uint8")
}

// MenuItemType represents menu item link types
//
// i.e. post_type, taxonomy, custom
type MenuItemType string

const (
	// MenuItemTypePost is a link to a specific post or page
	MenuItemTypePost     MenuItemType = "post_type"

	// MenuItemTypeTaxonomy is a link to a category or post tag
	MenuItemTypeTaxonomy MenuItemType = "taxonomy"

	// MenuItemTypeCustom is a custom or external link
	MenuItemTypeCustom   MenuItemType = "custom"
)

// Taxonomy represents term taxonomy names
//
// i.e. categories, nav_menu, post_tag
type Taxonomy string

const (
	// TaxonomyCategory is for post categories
	TaxonomyCategory Taxonomy = "category"

	// TaxonomyNavMenu is for menu locations
	TaxonomyNavMenu  Taxonomy = "nav_menu"

	// TaxonomyPostTag is for post tags
	TaxonomyPostTag  Taxonomy = "post_tag"
)