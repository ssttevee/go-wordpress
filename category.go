package wordpress

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const CacheKeyCategory = "wp_category_%d"

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

func (cat *Category) GetChildId(slug string) (int64, error) {
	row := cat.wp.db.QueryRow("SELECT term_id FROM "+cat.wp.table("terms")+" as t "+
		"JOIN "+cat.wp.table("term_taxonomy")+" as tt ON t.term_id = tt.term_id "+
		"WHERE tt.parent = ? AND t.slug = ?", cat.Id, slug)

	var id int64
	if err := row.Scan(&id); err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	return id, nil
}

func (cat *Category) GetChildrenIds() ([]int64, error) {
	ret := []int64{cat.Id}

	ids := ret[:]
	for len(ids) > 0 {
		stmt := "SELECT term_id FROM " + cat.wp.table("term_taxonomy") + " WHERE parent IN ("
		var params []interface{}
		for _, id := range ids {
			stmt += "?,"
			params = append(params, id)
		}

		rows, err := cat.wp.db.Query(stmt[:len(stmt)-1]+")", params...)
		if err != nil {
			return nil, err
		}

		ids = make([]int64, 0)
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

func (wp *WordPress) GetCategoryIdBySlug(slug string) (int64, error) {
	parts := strings.Split(slug, "/")

	var catId int64 = 0
	for _, part := range parts {
		ids, err := wp.QueryTerms(&TermQueryOptions{
			Taxonomy:   TaxonomyCategory,
			Slug:       part,
			ParentIdIn: []int64{catId}})
		if err != nil {
			return 0, err
		}

		if len(ids) == 0 {
			return 0, nil
		}

		catId = ids[0]
	}

	return catId, nil
}

// GetCategories gets all category data from the database
func (wp *WordPress) GetCategories(categoryIds ...int64) ([]*Category, error) {
	if len(categoryIds) == 0 {
		return []*Category{}, nil
	}

	var ret []*Category
	keyMap, _ := wp.cacheGetMulti(CacheKeyCategory, categoryIds, &ret)

	if len(keyMap) > 0 {
		missedIds := make([]int64, 0, len(keyMap))
		for _, indices := range keyMap {
			missedIds = append(missedIds, categoryIds[indices[0]])
		}

		terms, err := wp.GetTerms(missedIds...)
		if err != nil {
			return nil, err
		}

		counter := 0
		done := make(chan error)

		for _, term := range terms {
			c := Category{Term: *term}

			if c.Parent > 0 {
				counter++
				go func() {
					parents, err := wp.GetCategories(c.Parent)
					if err != nil {
						done <- fmt.Errorf("failed to get parent category for %d: %d\n%v", c.Id, c.Parent, err)
						return
					}

					if len(parents) == 0 {
						done <- fmt.Errorf("parent category for %d not found: %d", c.Id, c.Parent)
						return
					}

					c.Link = parents[0].Link + "/" + c.Slug

					done <- nil
				}()
			} else {
				c.Link = "/category/" + c.Slug
			}

			// insert into return set
			for _, index := range keyMap[fmt.Sprintf(CacheKeyCategory, c.Id)] {
				ret[index] = &c
			}
		}

		for ; counter > 0; counter-- {
			if err := <-done; err != nil {
				return nil, err
			}
		}

		// just let this run, no callback is needed
		go wp.cacheSetByKeyMap(keyMap, ret)
	}

	return ret, nil
}
