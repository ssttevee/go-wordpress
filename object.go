package wordpress

import (
	"cloud.google.com/go/trace"
	"encoding/base64"
	"fmt"
	"github.com/elgris/sqrl"
	"golang.org/x/net/context"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var regexpQuerySeparators = regexp.MustCompile("[,+~]")
var regexpQueryDelimiter = regexp.MustCompile("[^a-zA-Z]")

// Object represents a WordPress 'post' object
//
// Not really a Post object, per se, since WP uses it for other things like pages and menu items.
// However it's in the 'posts' table, so... whatever...
type Object struct {
	// The post's ID
	Id int64 `json:"id"`

	// The post author's ID
	AuthorId int64 `json:"author"`

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
	After string `param:"after"`
	Limit int    `param:"limit"`

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

	Meta      string   `param:"meta"`
	MetaAnd   []string `param:"meta__and"`
	MetaIn    []string `param:"meta__in"`
	MetaNotIn []string `param:"meta__not_in"`

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

	AfterDate time.Time
}

// GetMeta gets the object's metadata from the database
//
// Returns all metadata if no metadata keys are given
func (obj *Object) GetMeta(c context.Context, keys ...string) (map[string]string, error) {
	span := trace.FromContext(c).NewChild("/wordpress.Object.GetMeta")
	defer span.Finish()

	q := sqrl.Select("meta_key", "meta_value").
		From(table(c, "postmeta")).
		Where(sqrl.Eq{"post_id": obj.Id})

	if len(keys) > 0 {
		q = q.Where(sqrl.Eq{"meta_key": keys})
	}

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	trace.FromContext(c).SetLabel("wp/meta/query", sql)

	rows, err := database(c).Query(sql, args...)
	if err != nil {
		return nil, err
	}

	meta := make(map[string]string)
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			return nil, fmt.Errorf("Object GetMeta - Scan: %v", err)
		}

		meta[key] = val
	}

	span.SetLabel("wp/meta/count", strconv.Itoa(len(meta)))

	return meta, nil
}

// GetTaxonomy gets all term ids related to the object
// whose taxonomies match any of the given taxonomies
//
// i.e. `GetTaxonomy("category")` will return all of the object's related categories
func (obj *Object) GetTaxonomy(c context.Context, taxonomy ...Taxonomy) (Iterator, error) {
	if len(taxonomy) == 0 {
		return zeroIter, nil
	} else {
		return queryTerms(c, &TermQueryOptions{ObjectId: obj.Id, TaxonomyIn: taxonomy})
	}
}

// GetObjects gets all object data from the database
// (not including metadata)
func getObjects(c context.Context, objectIds ...int64) ([]*Object, error) {
	if len(objectIds) == 0 {
		return nil, nil
	}

	// dedupe the given object ids
	ids, idMap := dedupe(objectIds)

	// select objects from the database
	stmt, args, err := sqrl.Select("*").
		From(table(c, "posts")).
		Where(sqrl.Eq{"ID": ids}).ToSql()
	if err != nil {
		return nil, err
	}

	trace.FromContext(c).SetLabel("wp/object/query", stmt)

	rows, err := database(c).Query(stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("GetObjects - Query: %v", err)
	}

	ret := make([]*Object, len(objectIds))
	for rows.Next() {
		var obj Object
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
			return nil, fmt.Errorf("unable to read object data: %v", err)
		}

		obj.CommentStatus = commentStatus == "open"
		obj.PingStatus = pingStatus == "open"

		// redupe and insert into return set
		for _, index := range idMap[obj.Id] {
			ret[index] = &obj
		}
	}

	trace.FromContext(c).SetLabel("wp/object/count", strconv.Itoa(len(ret)))

	var mre MissingResourcesError
	for i, obj := range ret {
		if obj == nil {
			mre = append(mre, objectIds[i])
		}
	}

	if len(mre) > 0 {
		return nil, err
	}

	return ret, nil
}

type inSubquery struct {
	column string
	query  sqrl.Sqlizer
	neg    bool
}

func (in inSubquery) ToSql() (string, []interface{}, error) {
	stmt, args, err := in.query.ToSql()
	if err != nil {
		return "", nil, err
	}

	stmt = " IN (" + stmt + ")"
	if in.neg {
		stmt = " NOT" + stmt
	}

	return in.column + stmt, args, nil
}

// queryObjects returns the ids of the objects that match the query
func queryObjects(c context.Context, opts *ObjectQueryOptions) (Iterator, error) {
	if opts.Order == "" {
		opts.Order = "post_date"
	} else {
		// gotta prevent dat sql injection :)
		opts.Order = strings.Replace(opts.Order, "`", "", -1)
	}

	opts.Order = "`" + opts.Order + "`"

	q := sqrl.Select("ID", opts.Order).From(table(c, "posts"))

	termsSubQuery := sqrl.Select("object_id").
		From(table(c, "term_relationships") + " AS tr").
		Join(table(c, "term_taxonomy") + " AS tt ON tr.term_taxonomy_id = tt.term_taxonomy_id").
		Join(table(c, "terms") + " AS t ON tt.term_id = t.term_id")

	if opts.PostType != "" {
		q = q.Where(sqrl.Eq{"post_type": string(opts.PostType)})
	}

	if opts.PostStatus != "" {
		q = q.Where(sqrl.Eq{"post_status": string(opts.PostStatus)})
	}

	if opts.Author > 0 {
		q = q.Where(sqrl.Eq{"post_author": opts.Author})
	} else if opts.AuthorIn != nil && len(opts.AuthorIn) > 0 {
		q = q.Where(sqrl.Eq{"post_author": opts.AuthorIn})
	} else if opts.AuthorNotIn != nil && len(opts.AuthorNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"post_author": opts.AuthorNotIn})
	}

	if opts.AuthorName != "" {
		q = q.Where(inSubquery{
			column: "post_author",
			query: sqrl.Select("ID").
				From(table(c, "users")).
				Where(sqrl.Eq{"user_nicename": opts.AuthorName})})
	} else if opts.AuthorNameIn != nil && len(opts.AuthorNameIn) > 0 {
		q = q.Where(inSubquery{
			column: "post_author",
			query: sqrl.Select("ID").
				From(table(c, "users")).
				Where(sqrl.Eq{"user_nicename": opts.AuthorNameIn})})
	} else if opts.AuthorNameNotIn != nil && len(opts.AuthorNameNotIn) > 0 {
		q = q.Where(inSubquery{
			column: "post_author",
			query: sqrl.Select("ID").
				From(table(c, "users")).
				Where(sqrl.Eq{"user_nicename": opts.AuthorNameNotIn}),
			neg: true})
	}

	if opts.CategoryName != "" {
		sortCategory := func(cat string) {
			switch cat[:1] {
			case "+":
				opts.CategoryNameAnd = append(opts.CategoryNameAnd, cat[1:])
			case "~":
				opts.CategoryNameNotIn = append(opts.CategoryNameNotIn, cat[1:])
			default:
				if cat[:1] == "," {
					cat = cat[1:]
				}

				opts.CategoryNameIn = append(opts.CategoryNameIn, cat)
			}
		}
		prevIndex := 0
		for _, indices := range regexpQuerySeparators.FindAllStringIndex(opts.CategoryName, -1) {
			sortCategory(opts.CategoryName[prevIndex:indices[0]])
			prevIndex = indices[0]
		}

		sortCategory(opts.CategoryName[prevIndex:])

		opts.CategoryName = ""
	}

	if opts.CategoryNameAnd != nil && len(opts.CategoryNameAnd) > 0 {
		for _, categoryName := range opts.CategoryNameAnd {
			catId, _ := GetCategoryIdBySlug(c, categoryName)
			if catId == 0 {
				continue
			}
			opts.CategoryAnd = append(opts.CategoryAnd, catId)
		}
	} else if opts.CategoryNameIn != nil && len(opts.CategoryNameIn) > 0 {
		for _, categoryName := range opts.CategoryNameIn {
			catId, _ := GetCategoryIdBySlug(c, categoryName)
			if catId == 0 {
				continue
			}
			opts.CategoryIn = append(opts.CategoryIn, catId)
		}
	} else if opts.CategoryNameNotIn != nil && len(opts.CategoryNameNotIn) > 0 {
		for _, categoryName := range opts.CategoryNameNotIn {
			catId, _ := GetCategoryIdBySlug(c, categoryName)
			if catId == 0 {
				continue
			}
			opts.CategoryNotIn = append(opts.CategoryNotIn, catId)
		}
	}

	if opts.Category > 0 {
		cat := Category{Term: Term{Id: opts.Category}}
		ids, err := cat.GetChildrenIds(c)
		if err != nil {
			return nil, err
		}

		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "category",
				"t.term_id":   ids})})
	} else if opts.CategoryAnd != nil && len(opts.CategoryAnd) > 0 {
		for _, categoryId := range opts.CategoryAnd {
			cat := Category{Term: Term{Id: categoryId}}
			ids, err := cat.GetChildrenIds(c)
			if err != nil {
				return nil, err
			}

			q = q.Where(inSubquery{
				column: "ID",
				query: termsSubQuery.Where(sqrl.Eq{
					"tt.taxonomy": "category",
					"t.term_id":   ids})})
		}
	} else if opts.CategoryIn != nil && len(opts.CategoryIn) > 0 {
		var catIds []int64
		for _, categoryId := range opts.CategoryIn[:] {
			cat := Category{Term: Term{Id: categoryId}}
			ids, err := cat.GetChildrenIds(c)
			if err != nil {
				return nil, err
			}

			catIds = append(append(catIds, categoryId), ids...)
		}

		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "category",
				"t.term_id":   catIds})})
	} else if opts.CategoryNotIn != nil && len(opts.CategoryNotIn) > 0 {
		var catIds []int64
		for _, categoryId := range opts.CategoryNotIn[:] {
			cat := Category{Term: Term{Id: categoryId}}
			ids, err := cat.GetChildrenIds(c)
			if err != nil {
				return nil, err
			}

			catIds = append(append(catIds, categoryId), ids...)
		}

		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "category",
				"t.term_id":   catIds}),
			neg: true})
	}

	if opts.MenuId > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "nav_menu",
				"t.term_id":   opts.MenuId})})
	} else if opts.MenuIdAnd != nil && len(opts.MenuIdAnd) > 0 {
		for _, menuId := range opts.MenuIdAnd {
			q = q.Where(inSubquery{
				column: "ID",
				query: termsSubQuery.Where(sqrl.Eq{
					"tt.taxonomy": "nav_menu",
					"t.term_id":   menuId})})
		}
	} else if opts.MenuIdIn != nil && len(opts.MenuIdIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "nav_menu",
				"t.term_id":   opts.MenuIdIn})})
	} else if opts.MenuIdNotIn != nil && len(opts.MenuIdNotIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "nav_menu",
				"t.term_id":   opts.MenuIdNotIn}),
			neg: true})
	}

	if opts.MenuName != "" {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "nav_menu",
				"t.slug":      opts.MenuName})})
	} else if opts.MenuNameIn != nil && len(opts.MenuNameIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "nav_menu",
				"t.slug":      opts.MenuNameIn})})
	} else if opts.MenuNameNotIn != nil && len(opts.MenuNameNotIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "nav_menu",
				"t.slug":      opts.MenuNameNotIn}),
			neg: true})
	}

	var searchMeta = func(metas ...string) {
		neg := false
		if metas[0] == "is not in" {
			neg = true
			metas = metas[1:]

			if len(metas) == 0 {
				return
			}
		}
		subQuery := sqrl.Select("post_id").Distinct().From(table(c, "postmeta"))
		var metaConds sqrl.Or
		for _, meta := range metas {
			metaCond := make(sqrl.Eq)
			if equal := strings.IndexRune(meta, '='); equal != -1 {
				metaCond["meta_value"] = meta[equal+1:]
				meta = meta[:equal]
			}
			metaCond["meta_key"] = meta
			metaConds = append(metaConds, metaCond)
		}

		q = q.Where(inSubquery{
			column: "ID",
			query:  subQuery.Where(metaConds),
			neg:    neg})
	}

	if opts.Meta != "" {
		searchMeta(opts.Meta)
	} else if len(opts.MetaAnd) > 0 {
		for _, meta := range opts.MetaAnd {
			searchMeta(meta)
		}
	} else if len(opts.MetaIn) > 0 {
		searchMeta(opts.MetaIn...)
	} else if len(opts.MetaNotIn) > 0 {
		searchMeta(append([]string{"is not in"}, opts.MetaNotIn...)...)
	}

	if opts.Name != "" {
		q = q.Where(sqrl.Eq{"post_name": opts.Name})
	} else if opts.NameIn != nil && len(opts.NameIn) > 0 {
		q = q.Where(sqrl.Eq{"post_name": opts.NameIn})
	} else if opts.NameNotIn != nil && len(opts.NameNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"post_name": opts.NameNotIn})
	}

	if opts.Parent > 0 {
		q = q.Where(sqrl.Eq{"post_parent": opts.Parent})
	} else if opts.ParentIn != nil && len(opts.ParentIn) > 0 {
		q = q.Where(sqrl.Eq{"post_parent": opts.ParentIn})
	} else if opts.ParentNotIn != nil && len(opts.ParentNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"post_parent": opts.ParentNotIn})
	}

	if opts.Post > 0 {
		q = q.Where(sqrl.Eq{"ID": opts.Post})
	} else if opts.PostIn != nil && len(opts.PostIn) > 0 {
		q = q.Where(sqrl.Eq{"ID": opts.PostIn})
	} else if opts.PostNotIn != nil && len(opts.PostNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"ID": opts.PostNotIn})
	}

	if opts.TagId > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "post_tag",
				"t.term_id":   opts.TagId})})
	} else if opts.TagIdAnd != nil && len(opts.TagIdAnd) > 0 {
		for _, tagId := range opts.TagIdAnd {
			q = q.Where(inSubquery{
				column: "ID",
				query: termsSubQuery.Where(sqrl.Eq{
					"tt.taxonomy": "post_tag",
					"t.term_id":   tagId})})
		}
	} else if opts.TagIdIn != nil && len(opts.TagIdIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "post_tag",
				"t.term_id":   opts.TagIdIn})})
	} else if opts.TagIdNotIn != nil && len(opts.TagIdNotIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "post_tag",
				"t.term_id":   opts.TagIdNotIn}),
			neg: true})
	}

	if opts.TagName != "" {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "post_tag",
				"t.slug":      opts.TagName})})
	} else if opts.TagNameAnd != nil && len(opts.TagNameAnd) > 0 {
		for _, tagName := range opts.TagNameAnd {
			q = q.Where(inSubquery{
				column: "ID",
				query: termsSubQuery.Where(sqrl.Eq{
					"tt.taxonomy": "post_tag",
					"t.slug":      tagName})})
		}
	} else if opts.TagNameIn != nil && len(opts.TagNameIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "post_tag",
				"t.slug":      opts.TagNameIn})})
	} else if opts.TagNameNotIn != nil && len(opts.TagNameNotIn) > 0 {
		q = q.Where(inSubquery{
			column: "ID",
			query: termsSubQuery.Where(sqrl.Eq{
				"tt.taxonomy": "post_tag",
				"t.slug":      opts.TagNameNotIn}),
			neg: true})
	}

	if opts.Query != "" {
		var pred string
		var args []interface{}
		for _, word := range regexpQueryDelimiter.Split(opts.Query, -1) {
			if len(word) > 2 {
				pred += "post_name LIKE ? OR post_title LIKE ? OR post_content LIKE ? OR "

				word = "%" + word + "%"
				args = append(args, word, word, word)
			}
		}

		if pred != "" {
			q = q.Where("("+pred[:len(pred)-4]+")", args...)
		}
	}

	if opts.Day > 0 {
		q = q.Where(sqrl.Eq{"DAYOFMONTH(post_date)": opts.Day})
	}

	if opts.Month > 0 {
		q = q.Where(sqrl.Eq{"MONTH(post_date)": opts.Month})
	}

	if opts.Year > 0 {
		q = q.Where(sqrl.Eq{"YEAR(post_date)": opts.Year})
	}

	if !opts.AfterDate.IsZero() {
		q = q.Where("post_date > ?", opts.AfterDate)
	}

	if opts.After != "" {
		// ignore `q.After` if any errors occur
		if b, err := base64.URLEncoding.DecodeString(opts.After); err == nil {

			pred := opts.Order
			if opts.OrderAscending {
				pred += ">"
			} else {
				pred += "<"
			}

			pred += " ?"

			q = q.Where(pred, string(b))
		}
	}

	order := opts.Order
	if opts.OrderAscending {
		order += " ASC"
	} else {
		order += " DESC"
	}

	q = q.OrderBy(order)

	if opts.Limit == 0 {
		opts.Limit = 10
	}

	if opts.Limit > 0 {
		q = q.Limit(uint64(opts.Limit))
	}

	stmt, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	trace.FromContext(c).SetLabel("wp/object/query", stmt)

	rows, err := database(c).Query(stmt, args...)
	if err != nil {
		return nil, err
	}

	var ids []int64
	var cursors []string
	for rows.Next() {
		var id int64
		var cursor string
		if err = rows.Scan(&id, &cursor); err != nil {
			return nil, err
		}

		ids = append(ids, id)
		cursors = append(cursors, cursor)
	}

	trace.FromContext(c).SetLabel("wp/object/count", strconv.Itoa(len(ids)))

	it := iteratorImpl{cursor: opts.After}

	var counter int
	it.next = func() (id int64, err error) {
		if counter < len(ids) {
			id = ids[counter]
			it.cursor = base64.URLEncoding.EncodeToString([]byte(cursors[counter]))
			counter++
		} else {
			return it.exit(Done)
		}

		return id, err
	}

	return &it, nil
}
