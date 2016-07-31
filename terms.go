package wordpress

import (
	"fmt"
	"strconv"
)

const termCacheKey = "wp_term_%d"

// Term represents a WordPress term
type Term struct {
	// Term ID.
	Id int64 `json:"id"`

	// The term's name.
	Name string `json:"name"`

	// The term's slug.
	Slug string `json:"slug"`

	// The term's term_group.
	Group int64 `json:"group"`

	// Term Taxonomy ID.
	TaxonomyId int64 `json:"-"`

	// The term's taxonomy name.
	Taxonomy string `json:"taxonomy"`

	// The term's description.
	Description string `json:"description"`

	// ID of a term's parent term.
	Parent int64 `json:"parent"`

	// Cached object count for this term.
	Count int64 `json:"-"`
}

// TermQueryOptions represents the available parameters for querying
type TermQueryOptions struct {
	Page    int `param:"page"`
	PerPage int `param:"per_page"`

	Id      int64   `param:"term_id"`
	IdIn    []int64 `param:"term_id__in"`
	IdNotIn []int64 `param:"term_id__not_in"`

	Name      string   `param:"term_name"`
	NameIn    []string `param:"term_name__in"`
	NameNotIn []string `param:"term_name__not_in"`

	ObjectId      int64   `param:"object_id"`
	ObjectIdIn    []int64 `param:"object_id__in"`
	ObjectIdNotIn []int64 `param:"object_id__not_in"`

	Slug      string   `param:"term_slug"`
	SlugIn    []string `param:"term_slug__in"`
	SlugNotIn []string `param:"term_slug__not_in"`

	Taxonomy      Taxonomy   `param:"taxonomy"`
	TaxonomyIn    []Taxonomy `param:"taxonomy__in"`
	TaxonomyNotIn []Taxonomy `param:"taxonomy__not_in"`
}

// GetTerms gets all term data from the database
func (wp *WordPress) GetTerms(termIds ...int64) ([]*Term, error) {
	if len(termIds) == 0 {
		return []*Term{}, nil
	}

	var ret []*Term
	keyMap, _ := wp.cacheGetMulti(termCacheKey, termIds, &ret)

	if len(keyMap) > 0 {
		params := make([]interface{}, 0, len(keyMap))
		stmt := "SELECT t.term_id, t.name, t.slug, t.term_group, tt.term_taxonomy_id, tt.taxonomy, tt.description, tt.parent, tt.count FROM " + wp.table("terms") + " AS t " +
			"JOIN (" + wp.table("term_taxonomy") + " AS tt) ON tt.term_id = t.term_id WHERE t.term_id IN ("
		for _, index := range keyMap {
			stmt += "?,"
			params = append(params, termIds[index])
		}
		stmt = stmt[:len(stmt)-1] + ")"

		rows, err := wp.db.Query(stmt, params...)
		if err != nil {
			return nil, fmt.Errorf("Term SQL query fail: %v", err)
		}

		keys := make([]string, 0, len(keyMap))
		toCache := make([]*Term, 0, len(keyMap))

		for rows.Next() {
			t := Term{}

			if err := rows.Scan(
				&t.Id,
				&t.Name,
				&t.Slug,
				&t.Group,
				&t.TaxonomyId,
				&t.Taxonomy,
				&t.Description,
				&t.Parent,
				&t.Count); err != nil {
				return nil, fmt.Errorf("Unable to read term data: %v", err)
			}

			// prepare for storing to cache
			key := fmt.Sprintf(termCacheKey, t.Id)

			keys = append(keys, key)
			toCache = append(toCache, &t)

			// insert into return set
			ret[keyMap[key]] = &t
		}

		// just let this run, no callback is needed
		go func() {
			_ = wp.cacheSetMulti(keys, toCache)
		}()
	}

	return ret, nil
}

// QueryTerms queries the database and returns all matching term ids
func (wp *WordPress) QueryTerms(q *TermQueryOptions) ([]int64, error) {
	stmt := "SELECT DISTINCT t.term_id FROM " + wp.table("terms") + " AS t " +
		"JOIN (" + wp.table("term_taxonomy") + " AS tt, " + wp.table("term_relationships") + " AS tr) " +
		"ON tt.term_id = t.term_id AND tr.term_taxonomy_id = tt.term_taxonomy_id "
	where := "WHERE "

	var params []interface{}

	if q.Name != "" {
		where += "t.name = ? AND "
		params = append(params, q.Name)
	} else if q.NameIn != nil && len(q.NameIn) > 0 {
		where += "t.name IN (?"
		params = append(params, q.NameIn[0])
		for _, name := range q.NameIn[1:] {
			where += ", ?"
			params = append(params, name)
		}
		where += ") AND "
	} else if q.NameNotIn != nil && len(q.NameNotIn) > 0 {
		where += "t.name NOT IN (?"
		params = append(params, q.NameNotIn[0])
		for _, name := range q.NameNotIn[1:] {
			where += ", ?"
			params = append(params, name)
		}
		where += ") AND "
	}

	if q.ObjectId > 0 {
		where += "tr.object_id = ? AND "
		params = append(params, q.ObjectId)
	} else if q.ObjectIdIn != nil && len(q.ObjectIdIn) > 0 {
		where += "tr.object_id IN (?"
		params = append(params, q.ObjectIdIn[0])
		for _, objectId := range q.ObjectIdIn[1:] {
			where += ", ?"
			params = append(params, objectId)
		}
		where += ") AND "
	} else if q.ObjectIdNotIn != nil && len(q.ObjectIdNotIn) > 0 {
		where += "tr.object_id NOT IN (?"
		params = append(params, q.ObjectIdNotIn[0])
		for _, objectId := range q.ObjectIdNotIn[1:] {
			where += ", ?"
			params = append(params, objectId)
		}
		where += ") AND "
	}

	if q.Slug != "" {
		where += "t.slug = ? AND "
		params = append(params, q.Slug)
	} else if q.SlugIn != nil && len(q.SlugIn) > 0 {
		where += "t.slug IN (?"
		params = append(params, q.SlugIn[0])
		for _, slug := range q.SlugIn[1:] {
			where += ", ?"
			params = append(params, slug)
		}
		where += ") AND "
	} else if q.SlugNotIn != nil && len(q.SlugNotIn) > 0 {
		where += "t.slug NOT IN (?"
		params = append(params, q.SlugNotIn[0])
		for _, slug := range q.SlugNotIn[1:] {
			where += ", ?"
			params = append(params, slug)
		}
		where += ") AND "
	}

	if q.Taxonomy != "" {
		where += "tt.taxonomy = ? AND "
		params = append(params, string(q.Taxonomy))
	} else if q.TaxonomyIn != nil && len(q.TaxonomyIn) > 0 {
		where += "tt.taxonomy IN (?"
		params = append(params, string(q.TaxonomyIn[0]))
		for _, taxonomy := range q.TaxonomyIn[1:] {
			where += ", ?"
			params = append(params, string(taxonomy))
		}
		where += ") AND "
	} else if q.TaxonomyNotIn != nil && len(q.TaxonomyNotIn) > 0 {
		where += "tt.taxonomy NOT IN (?"
		params = append(params, string(q.TaxonomyNotIn[0]))
		for _, taxonomy := range q.TaxonomyNotIn[1:] {
			where += ", ?"
			params = append(params, string(taxonomy))
		}
		where += ") AND "
	}

	if q.Id > 0 {
		where += "t.term_id = ? AND "
		params = append(params, q.Id)
	} else if q.IdIn != nil && len(q.IdIn) > 0 {
		where += "t.term_id IN (?"
		params = append(params, q.IdIn[0])
		for _, termId := range q.IdIn[1:] {
			where += ", ?"
			params = append(params, termId)
		}
		where += ") AND "
	} else if q.IdNotIn != nil && len(q.IdNotIn) > 0 {
		where += "t.term_id NOT IN (?"
		params = append(params, q.IdNotIn[0])
		for _, termId := range q.IdNotIn[1:] {
			where += ", ?"
			params = append(params, termId)
		}
		where += ") AND "
	}

	if where == "WHERE " {
		return []int64{}, nil
	}

	where = where[:len(where)-4]

	perPage := q.PerPage
	if perPage >= 0 {
		if perPage == 0 {
			perPage = 10
		}

		limit := "LIMIT " + strconv.Itoa(perPage) + " "

		if q.Page > 1 {
			limit += "OFFSET " + strconv.Itoa(q.Page*perPage) + " "
		}

		where += limit
	}

	rows, err := wp.db.Query(stmt+where, params...)
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
