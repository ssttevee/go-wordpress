package wordpress

import (
	"encoding/base64"
	"fmt"
	"github.com/elgris/sqrl"
	"go.opencensus.io/trace"
	"golang.org/x/net/context"
	"strconv"
)

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
	After string `param:"after"`
	Limit int    `param:"limit"`

	Order          string `param:"order_by"`
	OrderAscending bool   `param:"order_asc"`

	Id      int64   `param:"term_id"`
	IdIn    []int64 `param:"term_id__in"`
	IdNotIn []int64 `param:"term_id__not_in"`

	Name      string   `param:"term_name"`
	NameIn    []string `param:"term_name__in"`
	NameNotIn []string `param:"term_name__not_in"`

	ObjectId      int64   `param:"object_id"`
	ObjectIdIn    []int64 `param:"object_id__in"`
	ObjectIdNotIn []int64 `param:"object_id__not_in"`

	ParentId      int64   `param:"parent_id"`
	ParentIdIn    []int64 `param:"parent_id__in"`
	ParentIdNotIn []int64 `param:"parent_id__not_in"`

	Slug      string   `param:"term_slug"`
	SlugIn    []string `param:"term_slug__in"`
	SlugNotIn []string `param:"term_slug__not_in"`

	Taxonomy      Taxonomy   `param:"taxonomy"`
	TaxonomyIn    []Taxonomy `param:"taxonomy__in"`
	TaxonomyNotIn []Taxonomy `param:"taxonomy__not_in"`
}

// GetTerms gets all term data from the database
func getTerms(c context.Context, termIds ...int64) ([]*Term, error) {
	if len(termIds) == 0 {
		return []*Term{}, nil
	}

	ids, idMap := dedupe(termIds)

	stmt, args, err := sqrl.Select("t.term_id", "t.name", "t.slug", "t.term_group", "tt.term_taxonomy_id", "tt.taxonomy", "tt.description", "tt.parent", "tt.count").
		From(table(c, "terms") + " AS t").
		Join(table(c, "term_taxonomy") + " AS tt ON tt.term_id = t.term_id").
		Where(sqrl.Eq{"t.term_id": ids}).ToSql()
	if err != nil {
		return nil, err
	}

	trace.FromContext(c).AddAttributes(trace.StringAttribute("wp/term/query", stmt))

	rows, err := database(c).Query(stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("Term SQL query fail: %v", err)
	}

	ret := make([]*Term, len(termIds))
	for rows.Next() {
		var t Term
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
			return nil, fmt.Errorf("unable to read term data: %v", err)
		}

		// redupe and insert into return set
		for _, index := range idMap[t.Id] {
			ret[index] = &t
		}
	}

	trace.FromContext(c).AddAttributes(trace.Int64Attribute("wp/term/count", int64(len(ret))))

	var mre MissingResourcesError
	for i, term := range ret {
		if term == nil {
			mre = append(mre, termIds[i])
		}
	}

	if len(mre) > 0 {
		return nil, err
	}

	return ret, nil
}

// QueryTerms returns the ids of the terms that match the query
func QueryTerms(c context.Context, opts *TermQueryOptions) (Iterator, error) {
	return queryTerms(c, opts)
}

func queryTerms(c context.Context, opts *TermQueryOptions) (Iterator, error) {
	order := opts.Order
	if len(order) == 0 {
		order = "t.term_id ASC"
	} else {
		if opts.OrderAscending {
			order += " ASC"
		} else {
			order += " DESC"
		}
	}

	q := sqrl.Select("t.term_id").
		From(table(c, "terms") + " AS t").
		OrderBy(order)

	var requireTaxonomy, requireRelationships bool

	if opts.Name != "" {
		q = q.Where(sqrl.Eq{"t.name": opts.Name})
	} else if opts.NameIn != nil && len(opts.NameIn) > 0 {
		q = q.Where(sqrl.Eq{"t.name": opts.NameIn})
	} else if opts.NameNotIn != nil && len(opts.NameNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"t.name": opts.NameNotIn})
	}

	if opts.ObjectId > 0 {
		requireRelationships = true
		q = q.Where(sqrl.Eq{"tr.object_id": opts.ObjectId})
	} else if opts.ObjectIdIn != nil && len(opts.ObjectIdIn) > 0 {
		requireRelationships = true
		q = q.Where(sqrl.Eq{"tr.object_id": opts.ObjectIdIn})
	} else if opts.ObjectIdNotIn != nil && len(opts.ObjectIdNotIn) > 0 {
		requireRelationships = true
		q = q.Where(sqrl.NotEq{"tr.object_id": opts.ObjectIdNotIn})
	}

	if opts.ParentId > 0 {
		requireTaxonomy = true
		q = q.Where(sqrl.Eq{"tt.parent": opts.ParentId})
	} else if opts.ParentIdIn != nil && len(opts.ParentIdIn) > 0 {
		requireTaxonomy = true
		q = q.Where(sqrl.Eq{"tt.parent": opts.ParentIdIn})
	} else if opts.ParentIdNotIn != nil && len(opts.ParentIdNotIn) > 0 {
		requireTaxonomy = true
		q = q.Where(sqrl.NotEq{"tt.parent": opts.ParentIdNotIn})
	}

	if opts.Slug != "" {
		q = q.Where(sqrl.Eq{"t.slug": opts.Slug})
	} else if opts.SlugIn != nil && len(opts.SlugIn) > 0 {
		q = q.Where(sqrl.Eq{"t.slug": opts.SlugIn})
	} else if opts.SlugNotIn != nil && len(opts.SlugNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"t.slug": opts.SlugNotIn})
	}

	if opts.Taxonomy != "" {
		requireTaxonomy = true
		q = q.Where(sqrl.Eq{"tt.taxonomy": string(opts.Taxonomy)})
	} else if opts.TaxonomyIn != nil && len(opts.TaxonomyIn) > 0 {
		requireTaxonomy = true
		var taxonomies []string
		for _, taxonomy := range opts.TaxonomyIn {
			taxonomies = append(taxonomies, string(taxonomy))
		}

		q = q.Where(sqrl.Eq{"tt.taxonomy": taxonomies})
	} else if opts.TaxonomyNotIn != nil && len(opts.TaxonomyNotIn) > 0 {
		requireTaxonomy = true
		var taxonomies []string
		for _, taxonomy := range opts.TaxonomyNotIn {
			taxonomies = append(taxonomies, string(taxonomy))
		}

		q = q.Where(sqrl.NotEq{"tt.taxonomy": taxonomies})
	}

	if opts.Id > 0 {
		q = q.Where(sqrl.Eq{"t.term_id": opts.Id})
	} else if opts.IdIn != nil && len(opts.IdIn) > 0 {
		q = q.Where(sqrl.Eq{"t.term_id": opts.IdIn})
	} else if opts.IdNotIn != nil && len(opts.IdNotIn) > 0 {
		q = q.Where(sqrl.NotEq{"t.term_id": opts.IdNotIn})
	}

	if opts.After != "" {
		// ignore `q.After` if any errors occur
		if b, err := base64.URLEncoding.DecodeString(opts.After); err == nil {
			pred := opts.Order
			if len(pred) == 0 {
				pred = "t.term_id"
			}

			if opts.OrderAscending {
				pred += ">"
			} else {
				pred += "<"
			}

			pred += " ?"

			q = q.Where(pred, string(b))
		}
	}

	q = q.OrderBy(order)

	if opts.Limit == 0 {
		opts.Limit = 10
	}

	if opts.Limit >= 0 {
		q = q.Limit(uint64(opts.Limit))
	}

	if requireTaxonomy || requireRelationships {
		q = q.Join(table(c, "term_taxonomy") + " AS tt ON tt.term_id = t.term_id")
	}

	if requireRelationships {
		q = q.Join(table(c, "term_relationships") + " AS tr ON tr.term_taxonomy_id = tt.term_taxonomy_id")
	}

	sql, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	trace.FromContext(c).AddAttributes(trace.StringAttribute("wp/term/query", sql))

	rows, err := database(c).Query(sql, args...)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	trace.FromContext(c).AddAttributes(trace.Int64Attribute("wp/term/count", int64(len(ids))))

	it := iteratorImpl{cursor: opts.After}

	var counter int
	it.next = func() (id int64, err error) {
		if counter < len(ids) {
			id = ids[counter]
			it.cursor = base64.URLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
			counter++
		} else {
			return it.exit(Done)
		}

		return id, err
	}

	return &it, nil
}
