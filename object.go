package wordpress

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const CacheKeyObject = "wp_object_%d"

var regexpQuerySeparators = regexp.MustCompile("[,+~]")
var regexpQueryDelimiter = regexp.MustCompile("[^a-zA-Z]")

// Object represents a WordPress 'post' object
//
// Not really a Post object, per se, since WP uses it for other things like pages and menu items.
// However it's in the 'posts' table, so... whatever...
type Object struct {
	wp *WordPress

	// The post's ID
	Id int64 `json:"id"`

	// The post author's ID
	AuthorId int `json:"author"`

	// The post's local publication time.
	Date time.Time `json:"date"`

	// The post's GMT publication time.
	DateGmt time.Time `json:"-"`

	// The post's content.
	Content string `json:"content"`

	// The post's title.
	Title string `json:"title"`

	// The post's excerpt.
	Excerpt string `json:"excerpt"`

	// The post's status.
	Status PostStatus `json:"status"`

	// Whether comments are allowed.
	CommentStatus bool `json:"comment_status"`

	// Whether pings are allowed.
	PingStatus bool `json:"ping_status"`

	// The post's password in plain text.
	Password string `json:"-"`

	// The post's slug.
	Name string `json:"slug"`

	// URLs queued to be pinged.
	ToPing URLList `json:"-"`

	// URLs that have been pinged.
	Pinged URLList `json:"-"`

	// The post's local modified time.
	Modified time.Time `json:"modified"`

	// The post's GMT modified time.
	ModifiedGmt time.Time `json:"-"`

	// A utility field for post content.
	ContentFiltered string `json:"-"`

	// The post's parent post.
	ParentId int `json:"parent"`

	// The post's unique identifier, not necessarily a URL, used as the feed GUID.
	Guid string `json:"-"`

	// A field used for ordering posts.
	MenuOrder int `json:"-"`

	// The post's type. (i.e. post or page)
	Type string `json:"type"`

	// An attachment's mime type.
	MimeType string `json:"mime_type,omitempty"`

	// Cached comment count.
	CommentCount int `json:"-"`
}

// ObjectQueryOptions represents the available parameters for querying
//
// Somewhat similar to WP's json plugin
type ObjectQueryOptions struct {
	Page    int `param:"page"`
	PerPage int `param:"per_page"`

	Order          string `param:"order_by"`
	OrderAscending bool   `param:"order_asc"`

	PostType   PostType   `param:"post_type"`
	PostStatus PostStatus `param:"post_status"`

	Author      int64   `param:"author_id"`
	AuthorIn    []int64 `param:"author_id__in"`
	AuthorNotIn []int64 `param:"author_id__not_in"`

	AuthorName      string   `param:"author_name"`
	AuthorNameIn    []string `param:"author_name__in"`
	AuthorNameNotIn []string `param:"author_name__not_in"`

	Category      int64   `param:"category_id"`
	CategoryAnd   []int64 `param:"category_id__and"`
	CategoryIn    []int64 `param:"category_id__in"`
	CategoryNotIn []int64 `param:"category_id__not_in"`

	CategoryName      string   `param:"category_name"`
	CategoryNameAnd   []string `param:"category_name__and"`
	CategoryNameIn    []string `param:"category_name__in"`
	CategoryNameNotIn []string `param:"category_name__not_in"`

	MenuId      int64   `param:"menu_id"`
	MenuIdAnd   []int64 `param:"menu_id__and_in"`
	MenuIdIn    []int64 `param:"menu_id__in"`
	MenuIdNotIn []int64 `param:"menu_id__not_in"`

	MenuName      string   `param:"menu_name"`
	MenuNameAnd   []string `param:"menu_name__and"`
	MenuNameIn    []string `param:"menu_name__in"`
	MenuNameNotIn []string `param:"menu_name__not_in"`

	Name      string   `param:"post_name"`
	NameIn    []string `param:"post_name__in"`
	NameNotIn []string `param:"post_name__not_in"`

	Parent      int64   `param:"post_parent"`
	ParentIn    []int64 `param:"post_parent__in"`
	ParentNotIn []int64 `param:"post_parent__not_in"`

	Post      int64   `param:"post_id"`
	PostIn    []int64 `param:"post_id__in"`
	PostNotIn []int64 `param:"post_id__not_in"`

	TagId      int64   `param:"tag_id"`
	TagIdAnd   []int64 `param:"tag_id__and"`
	TagIdIn    []int64 `param:"tag_id__in"`
	TagIdNotIn []int64 `param:"tag_id__not_in"`

	TagName      string   `param:"tag_name"`
	TagNameAnd   []string `param:"tag_name__and"`
	TagNameIn    []string `param:"tag_name__in"`
	TagNameNotIn []string `param:"tag_name__not_in"`

	Query string `param:"q"`

	Day   int `param:"day_of_month"`
	Month int `param:"month_num"`
	Year  int `param:"year"`
}

// GetMeta gets the object's metadata from the database
//
// Returns all metadata if no metadata keys are given
func (obj *Object) GetMeta(keys ...string) (map[string]string, error) {
	params := make([]interface{}, 0, 1)
	stmt := "SELECT meta_key, meta_value FROM " + obj.wp.table("postmeta") + " " +
		"WHERE post_id = ?"
	params = append(params, obj.Id)

	if len(keys) > 0 {
		stmt += " AND meta_key IN (?"
		params = append(params, keys[0])
		for _, key := range keys[1:] {
			stmt += ",?"
			params = append(params, key)
		}
		stmt += ")"
	}

	rows, err := obj.wp.db.Query(stmt, params...)
	if err != nil {
		return nil, fmt.Errorf("Object GetMeta - Query: %v", err)
	}

	meta := make(map[string]string)
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			return nil, fmt.Errorf("Object GetMeta - Scan: %v", err)
		}

		meta[key] = val
	}

	return meta, nil
}

// GetTaxonomy gets all term ids related to the object
// whose taxonomies match any of the given taxonomies
//
// i.e. `GetTaxonomy("category")` will return all of the object's related categories
func (obj *Object) GetTaxonomy(taxonomy ...Taxonomy) ([]int64, error) {
	if len(taxonomy) == 0 {
		return []int64{}, nil
	} else if len(taxonomy) == 1 {
		return obj.wp.QueryTerms(&TermQueryOptions{ObjectId: obj.Id, Taxonomy: taxonomy[0]})
	} else {
		return obj.wp.QueryTerms(&TermQueryOptions{ObjectId: obj.Id, TaxonomyIn: taxonomy})
	}
}

// GetObjects gets all object data from the database
// (not including metadata)
func (wp *WordPress) GetObjects(objectIds ...int64) ([]*Object, error) {
	if len(objectIds) == 0 {
		return []*Object{}, nil
	}

	var ret []*Object
	keyMap, _ := wp.cacheGetMulti(CacheKeyObject, objectIds, &ret)

	if len(keyMap) > 0 {
		params := make([]interface{}, 0, len(keyMap))
		stmt := "SELECT * FROM " + wp.table("posts") + " " +
			"WHERE ID IN ("
		for _, index := range keyMap {
			stmt += "?,"
			params = append(params, objectIds[index])
		}
		stmt = stmt[:len(stmt)-1] + ") "

		rows, err := wp.db.Query(stmt, params...)
		if err != nil {
			return nil, fmt.Errorf("GetObjects - Query: %v", err)
		}

		keys := make([]string, 0, len(keyMap))
		toCache := make([]*Object, 0, len(keyMap))

		for rows.Next() {
			obj := Object{}

			var commentStatus, pingStatus string

			if err := rows.Scan(
				&obj.Id,
				&obj.AuthorId,
				&obj.Date,
				&obj.DateGmt,
				&obj.Content,
				&obj.Title,
				&obj.Excerpt,
				&obj.Status,
				&commentStatus,
				&pingStatus,
				&obj.Password,
				&obj.Name,
				&obj.ToPing,
				&obj.Pinged,
				&obj.Modified,
				&obj.ModifiedGmt,
				&obj.ContentFiltered,
				&obj.ParentId,
				&obj.Guid,
				&obj.MenuOrder,
				&obj.Type,
				&obj.MimeType,
				&obj.CommentCount); err != nil {
				return nil, fmt.Errorf("GetObjects - Scan: %v", err)
			}

			obj.CommentStatus = commentStatus == "open"
			obj.PingStatus = pingStatus == "open"

			// prepare for storing to cache
			key := fmt.Sprintf(CacheKeyObject, obj.Id)

			keys = append(keys, key)
			toCache = append(toCache, &obj)

			// insert into return set
			ret[keyMap[key]] = &obj
		}

		// just let this run, no callback is needed
		go func() {
			_ = wp.cacheSetMulti(keys, toCache)
		}()
	}

	for _, obj := range ret {
		obj.wp = wp
	}

	return ret, nil
}

// QueryObjects queries the database and returns all matching object ids
func (wp *WordPress) QueryObjects(q *ObjectQueryOptions) ([]int64, error) {
	stmt := "SELECT DISTINCT ID FROM " + wp.table("posts") + " "
	where := "WHERE "

	var params []interface{}

	termSearchSetup := "SELECT object_id FROM " + wp.table("term_relationships") + " AS tr " +
		"INNER JOIN (" + wp.table("terms") + " AS t, " + wp.table("term_taxonomy") + " AS tt) " +
		"ON tr.term_taxonomy_id = tt.term_taxonomy_id AND tt.term_id = t.term_id WHERE "

	if q.PostType != "" {
		where += "post_type = ? AND "
		params = append(params, string(q.PostType))
	}

	if q.PostStatus != "" {
		where += "post_status = ? AND "
		params = append(params, string(q.PostStatus))
	}

	if q.Author > 0 {
		where += "post_author = ? AND "
		params = append(params, q.Author)
	} else if q.AuthorIn != nil && len(q.AuthorIn) > 0 {
		where += "post_author IN (?"
		params = append(params, q.AuthorIn[0])
		for _, authorId := range q.AuthorIn[1:] {
			where += ", ?"
			params = append(params, authorId)
		}
		where += ") AND "
	} else if q.AuthorNotIn != nil && len(q.AuthorNotIn) > 0 {
		where += "post_author NOT IN (?"
		params = append(params, q.AuthorNotIn[0])
		for _, authorId := range q.AuthorNotIn[1:] {
			where += ", ?"
			params = append(params, authorId)
		}
		where += ") AND "
	}

	if q.AuthorName != "" {
		where += "post_author IN (SELECT ID FROM wp_users WHERE user_nicename = ?) AND "
		params = append(params, q.AuthorName)
	} else if q.AuthorNameIn != nil && len(q.AuthorNameIn) > 0 {
		where += "post_author IN (SELECT ID FROM wp_users WHERE user_nicename IN (?"
		params = append(params, q.AuthorNameIn[0])
		for _, authorName := range q.AuthorNameIn[1:] {
			where += ", ?"
			params = append(params, authorName)
		}
		where += ")) AND "
	} else if q.AuthorNameNotIn != nil && len(q.AuthorNameNotIn) > 0 {
		where += "post_author NOT IN (SELECT ID FROM wp_users WHERE user_nicename NOT IN (?"
		params = append(params, q.AuthorNameNotIn[0])
		for _, authorName := range q.AuthorNameNotIn[1:] {
			where += ", ?"
			params = append(params, authorName)
		}
		where += ")) AND "
	}

	if q.CategoryName != "" {
		sortCategory := func(cat string) {
			switch cat[:1] {
			case "+":
				if q.CategoryNameAnd == nil {
					q.CategoryNameAnd = make([]string, 0, 1)
				}

				q.CategoryNameAnd = append(q.CategoryNameAnd, cat[1:])
			case "~":
				if q.CategoryNameNotIn == nil {
					q.CategoryNameNotIn = make([]string, 0, 1)
				}

				q.CategoryNameNotIn = append(q.CategoryNameNotIn, cat[1:])
			default:
				if q.CategoryNameIn == nil {
					q.CategoryNameIn = make([]string, 0, 1)
				}

				if cat[:1] == "," {
					cat = cat[1:]
				}

				q.CategoryNameIn = append(q.CategoryNameIn, cat)
			}
		}
		prevIndex := 0
		for _, indices := range regexpQuerySeparators.FindAllStringIndex(q.CategoryName, -1) {
			sortCategory(q.CategoryName[prevIndex:indices[0]])
			prevIndex = indices[0]
		}

		sortCategory(q.CategoryName[prevIndex:])

		q.CategoryName = ""
	}

	if q.CategoryNameAnd != nil && len(q.CategoryNameAnd) > 0 {
		for _, categoryName := range q.CategoryNameAnd {
			catId, _ := wp.GetCategoryIdBySlug(categoryName)
			if catId == 0 {
				return []int64{}, nil
			}
			q.CategoryAnd = append(q.CategoryAnd, catId)
		}
	} else if q.CategoryNameIn != nil && len(q.CategoryNameIn) > 0 {
		for _, categoryName := range q.CategoryNameIn {
			catId, _ := wp.GetCategoryIdBySlug(categoryName)
			if catId == 0 {
				return []int64{}, nil
			}
			q.CategoryIn = append(q.CategoryIn, catId)
		}
	} else if q.CategoryNameNotIn != nil && len(q.CategoryNameNotIn) > 0 {
		for _, categoryName := range q.CategoryNameNotIn {
			catId, _ := wp.GetCategoryIdBySlug(categoryName)
			if catId == 0 {
				return []int64{}, nil
			}
			q.CategoryNotIn = append(q.CategoryNotIn, catId)
		}
	}

	if q.Category > 0 {
		cat := &Category{Term: Term{wp: wp, Id: q.Category}}

		ids, err := cat.GetChildrenIds()
		if err != nil {
			return nil, err
		}

		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'category' AND t.term_id IN ("
		for _, categoryId := range ids {
			where += "?,"
			params = append(params, categoryId)
		}
		where = where[:len(where)-1] + ")) AND "
	} else if q.CategoryAnd != nil && len(q.CategoryAnd) > 0 {
		for _, categoryId := range q.CategoryAnd {
			where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'category' AND t.term_id = ?) AND "
			params = append(params, categoryId)
		}
	} else if q.CategoryIn != nil && len(q.CategoryIn) > 0 {
		for _, categoryId := range q.CategoryIn[:] {
			cat := &Category{Term: Term{wp: wp, Id: categoryId}}

			ids, err := cat.GetChildrenIds()
			if err != nil {
				return nil, err
			}

			q.CategoryIn = append(q.CategoryIn, ids...)
		}

		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'category' AND t.term_id IN ("
		for _, categoryId := range q.CategoryIn {
			where += "?,"
			params = append(params, categoryId)
		}
		where = where[:len(where)-1] + ")) AND "
	} else if q.CategoryNotIn != nil && len(q.CategoryNotIn) > 0 {
		for _, categoryId := range q.CategoryNotIn[:] {
			cat := &Category{Term: Term{wp: wp, Id: categoryId}}

			ids, err := cat.GetChildrenIds()
			if err != nil {
				return nil, err
			}

			q.CategoryNotIn = append(q.CategoryNotIn, ids...)
		}

		where += "ID NOT IN (" + termSearchSetup + "tt.taxonomy = 'category' AND t.term_id IN (?"
		for _, categoryId := range q.CategoryNotIn {
			where += "?,"
			params = append(params, categoryId)
		}
		where = where[:len(where)-1] + ")) AND "
	}

	if q.MenuId > 0 {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.term_id = ?) AND "
		params = append(params, q.MenuId)
	} else if q.MenuIdAnd != nil && len(q.MenuIdAnd) > 0 {
		for _, menuId := range q.MenuIdAnd {
			where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.term_id = ?) AND "
			params = append(params, menuId)
		}
	} else if q.MenuIdIn != nil && len(q.MenuIdIn) > 0 {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.term_id IN (?"
		params = append(params, q.MenuIdIn[0])
		for _, menuId := range q.MenuIdIn[1:] {
			where += ", ?"
			params = append(params, menuId)
		}
		where += ")) AND "
	} else if q.MenuIdNotIn != nil && len(q.MenuIdNotIn) > 0 {
		where += "ID NOT IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.term_id IN (?"
		params = append(params, q.MenuIdNotIn[0])
		for _, menuId := range q.MenuIdNotIn[1:] {
			where += ", ?"
			params = append(params, menuId)
		}
		where += ")) AND "
	}

	if q.MenuName != "" {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.slug = ?) AND "
		params = append(params, q.MenuName)
	} else if q.MenuNameAnd != nil && len(q.MenuNameAnd) > 0 {
		for _, menuName := range q.MenuNameAnd {
			where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.slug = ?) AND "
			params = append(params, menuName)
		}
	} else if q.MenuNameIn != nil && len(q.MenuNameIn) > 0 {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.slug IN (?"
		params = append(params, q.MenuNameIn[0])
		for _, menuName := range q.MenuNameIn[1:] {
			where += ", ?"
			params = append(params, menuName)
		}
		where += ")) AND "
	} else if q.MenuNameNotIn != nil && len(q.MenuNameNotIn) > 0 {
		where += "ID NOT IN (" + termSearchSetup + "tt.taxonomy = 'nav_menu' AND t.slug IN (?"
		params = append(params, q.MenuNameNotIn[0])
		for _, menuName := range q.MenuNameNotIn[1:] {
			where += ", ?"
			params = append(params, menuName)
		}
		where += ")) AND "
	}

	if q.Name != "" {
		where += "post_name = ? AND "
		params = append(params, q.Name)
	} else if q.NameIn != nil && len(q.NameIn) > 0 {
		where += "post_name IN (?"
		params = append(params, q.NameIn[0])
		for _, name := range q.NameIn[1:] {
			where += ", ?"
			params = append(params, name)
		}
		where += ") AND "
	} else if q.NameNotIn != nil && len(q.NameNotIn) > 0 {
		where += "post_name NOT IN (?"
		params = append(params, q.NameNotIn[0])
		for _, name := range q.NameNotIn[1:] {
			where += ", ?"
			params = append(params, name)
		}
		where += ") AND "
	}

	if q.Parent > 0 {
		where += "post_parent = ? AND "
		params = append(params, q.Parent)
	} else if q.ParentIn != nil && len(q.ParentIn) > 0 {
		where += "post_parent IN (?"
		params = append(params, q.ParentIn[0])
		for _, parentId := range q.ParentIn[1:] {
			where += ", ?"
			params = append(params, parentId)
		}
		where += ") AND "
	} else if q.ParentNotIn != nil && len(q.ParentNotIn) > 0 {
		where += "post_parent NOT IN (?"
		params = append(params, q.ParentNotIn[0])
		for _, parentId := range q.ParentNotIn[1:] {
			where += ", ?"
			params = append(params, parentId)
		}
		where += ") AND "
	}

	if q.Post > 0 {
		where += "ID = ? AND "
		params = append(params, q.Post)
	} else if q.PostIn != nil && len(q.PostIn) > 0 {
		where += "ID IN (?"
		params = append(params, q.PostIn[0])
		for _, postId := range q.PostIn[1:] {
			where += ", ?"
			params = append(params, postId)
		}
		where += ") AND "
	} else if q.PostNotIn != nil && len(q.PostNotIn) > 0 {
		where += "ID NOT IN (?"
		params = append(params, q.PostNotIn[0])
		for _, postId := range q.PostNotIn[1:] {
			where += ", ?"
			params = append(params, postId)
		}
		where += ") AND "
	}

	if q.TagId > 0 {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.term_id = ?) AND "
		params = append(params, q.TagId)
	} else if q.TagIdAnd != nil && len(q.TagIdAnd) > 0 {
		for _, tagId := range q.TagIdAnd {
			where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.term_id = ?) AND "
			params = append(params, tagId)
		}
	} else if q.TagIdIn != nil && len(q.TagIdIn) > 0 {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.term_id IN (?"
		params = append(params, q.TagIdIn[0])
		for _, tagId := range q.TagIdIn[1:] {
			where += ", ?"
			params = append(params, tagId)
		}
		where += ")) AND "
	} else if q.TagIdNotIn != nil && len(q.TagIdNotIn) > 0 {
		where += "ID NOT IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.term_id IN (?"
		params = append(params, q.TagIdNotIn[0])
		for _, tagId := range q.TagIdNotIn[1:] {
			where += ", ?"
			params = append(params, tagId)
		}
		where += ")) AND "
	}

	if q.TagName != "" {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.slug = ?) AND "
		params = append(params, q.TagName)
	} else if q.TagNameAnd != nil && len(q.TagNameAnd) > 0 {
		for _, tagName := range q.TagNameAnd {
			where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.slug = ?) AND "
			params = append(params, tagName)
		}
	} else if q.TagNameIn != nil && len(q.TagNameIn) > 0 {
		where += "ID IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.slug IN (?"
		params = append(params, q.TagNameIn[0])
		for _, tagName := range q.TagNameIn[1:] {
			where += ", ?"
			params = append(params, tagName)
		}
		where += ")) AND "
	} else if q.TagNameNotIn != nil && len(q.TagNameNotIn) > 0 {
		where += "ID NOT IN (" + termSearchSetup + "tt.taxonomy = 'post_tag' AND t.slug IN (?"
		params = append(params, q.TagNameNotIn[0])
		for _, tagName := range q.TagNameNotIn[1:] {
			where += ", ?"
			params = append(params, tagName)
		}
		where += ")) AND "
	}

	if q.Query != "" {
		query := ""
		for _, word := range regexpQueryDelimiter.Split(q.Query, -1) {
			if len(word) > 2 {
				query += "post_name LIKE ? OR post_title LIKE ? OR post_content LIKE ? OR "

				word = "%" + word + "%"
				params = append(params, word, word, word)
			}
		}

		if query != "" {
			where += "(" + query[:len(query)-4] + ") AND "
		}
	}

	if q.Day > 0 {
		where += "DAYOFMONTH(post_date) = ? AND "
		params = append(params, q.Day)
	}

	if q.Month > 0 {
		where += "MONTH(post_date) = ? AND "
		params = append(params, q.Month)
	}

	if q.Year > 0 {
		where += "YEAR(post_date) = ? AND "
		params = append(params, q.Year)
	}

	if where == "WHERE " {
		where = ""
	} else {
		where = where[:len(where)-4]
	}

	order := "ORDER BY `"
	if q.Order != "" {
		// gotta prevent dat sql injection :)
		order += strings.Replace(q.Order, "`", "", -1)
	} else {
		order += "post_date"
	}
	order += "` "

	if q.OrderAscending {
		order += "ASC "
	} else {
		order += "DESC "
	}

	limit := ""
	perPage := q.PerPage
	if perPage >= 0 {
		if perPage == 0 {
			perPage = 10
		}

		limit += "LIMIT " + strconv.Itoa(perPage) + " "

		if q.Page > 1 {
			limit += "OFFSET " + strconv.Itoa(q.Page*perPage) + " "
		}
	}

	rows, err := wp.db.Query(stmt+where+order+limit, params...)
	if err != nil {
		return nil, err
	}

	var ret []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ret = append(ret, id)
	}

	return ret, nil
}
