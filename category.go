package wordpress

import (
	"cloud.google.com/go/trace"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elgris/sqrl"
	"golang.org/x/net/context"
	"strings"
)

// Category represents a WordPress category
type Category struct {
	Term

	Link string `json:"url"`
}

// MarshalJSON marshals itself into json
func (cat *Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":     cat.Id,
		"parent": cat.Parent,
		"name":   cat.Name,
		"url":    cat.Link})
}

// GetChildId returns the category id of the child looked up by it's slug
func (cat *Category) GetChildId(c context.Context, slug string) (int64, error) {
	span := trace.FromContext(c).NewChild("/wordpress.Category.GetChildId")
	defer span.Finish()

	stmt, args, err := sqrl.Select("term_id").
		From(table(c, "terms") + " AS t").
		Join(table(c, "term_taxonomy") + " AS tt ON t.term_id = tt.term_id").
		Where(sqrl.Eq{"tt.parent": cat.Id, "t.slug": slug}).ToSql()
	if err != nil {
		return 0, err
	}

	span.SetLabel("wp/query", stmt)

	var id int64
	if err := database(c).QueryRow(stmt, args...).Scan(&id); err != nil && err != sql.ErrNoRows {
		return 0, nil
	}

	return id, nil
}

// GetChildrenIds returns all the ids of the category and it's children
func (cat *Category) GetChildrenIds(c context.Context) ([]int64, error) {
	span := trace.FromContext(c).NewChild("/wordpress.Category.GetChildrenIds")
	defer span.Finish()

	ret := []int64{cat.Id}

	ids := ret[:]
	for len(ids) > 0 {
		stmt, args, err := sqrl.Select("term_id").
			From(table(c, "term_taxonomy")).
			Where(sqrl.Eq{"parent": ids}).ToSql()
		if err != nil {
			return nil, err
		}

		rows, err := database(c).Query(stmt, args...)
		if err != nil {
			return nil, err
		}

		ids = nil
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}

			ids = append(ids, id)
		}

		ret = append(ret, ids...)
	}

	return ret, nil
}

// GetCategoryIdBySlug returns the id of the category that matches the given slug
func GetCategoryIdBySlug(c context.Context, slug string) (int64, error) {
	span := trace.FromContext(c).NewChild("/wordpress.GetCategoryIdBySlug")
	defer span.Finish()

	c = trace.NewContext(c, span)

	parts := strings.Split(slug, "/")

	var catId int64
	for _, part := range parts {
		it, err := queryTerms(c, &TermQueryOptions{
			Taxonomy: TaxonomyCategory,
			Slug:     part,
			ParentId: catId})
		if err != nil {
			return 0, err
		}

		if catId, err = it.Next(); err != nil {
			return 0, errors.New("wordpress: non-existent category slug")
		}
	}

	return catId, nil
}

// GetCategories gets all category data from the database
func GetCategories(c context.Context, categoryIds ...int64) ([]*Category, error) {
	span := trace.FromContext(c).NewChild("/wordpress.GetCategories")
	defer span.Finish()

	c = trace.NewContext(c, span)

	if len(categoryIds) == 0 {
		return []*Category{}, nil
	}

	ids, idMap := dedupe(categoryIds)

	terms, err := getTerms(c, ids...)
	if err != nil {
		return nil, err
	}

	counter := 0
	done := make(chan error)

	ret := make([]*Category, len(categoryIds))
	for _, term := range terms {
		cat := Category{Term: *term}

		if cat.Parent > 0 {
			counter++
			go func() {
				parents, err := GetCategories(c, cat.Parent)
				if err != nil {
					done <- fmt.Errorf("failed to get parent category for %d: %d\n%v", cat.Id, cat.Parent, err)
					return
				}

				if len(parents) == 0 {
					done <- fmt.Errorf("parent category for %d not found: %d", cat.Id, cat.Parent)
					return
				}

				cat.Link = parents[0].Link + "/" + cat.Slug

				done <- nil
			}()
		} else {
			cat.Link = "/category/" + cat.Slug
		}

		// insert into return set
		for _, index := range idMap[cat.Id] {
			ret[index] = &cat
		}
	}

	for ; counter > 0; counter-- {
		if err := <-done; err != nil {
			return nil, err
		}
	}

	return ret, nil
}
