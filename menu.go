package wordpress

import (
	"fmt"
	"github.com/wulijun/go-php-serialize/phpserialize"
	"sort"
	"strconv"
)

// MenuItem represents a WordPress menu item
type MenuItem struct {
	Id       int64 `json:"id"`
	ParentId int64 `json:"-"`

	Order int `json:"-"`

	Title string `json:"title"`
	Link  string `json:"url"`

	Attr    string `json:"attrs,omitempty"`
	Classes string `json:"classes,omitempty"`
	Target  string `json:"target,omitempty"`

	ObjectId int64  `json:"object_id"`
	Object   string `json:"object"`

	Type MenuItemType `json:"type"`

	Xfn string `json:"xfn,omitempty"`

	Children []*MenuItem `json:"children,omitempty"`
}

// MenuLocation represents a WordPress menu location
type MenuLocation struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// GetMenuLocations gets the available menu locations from the database
func (wp *WordPress) GetMenuLocations() ([]*MenuLocation, error) {
	rows, err := wp.db.Query("SELECT t.term_id, t.name, t.slug FROM "+wp.table("terms")+" AS t "+
		"JOIN "+wp.table("term_taxonomy")+" AS tt ON t.term_id = tt.term_id "+
		"WHERE tt.taxonomy = ?", "nav_menu")
	if err != nil {
		return nil, err
	}

	var ret []*MenuLocation
	for rows.Next() {
		var ml MenuLocation

		if err := rows.Scan(&ml.Id, &ml.Name, &ml.Slug); err != nil {
			return nil, err
		}

		ret = append(ret, &ml)
	}

	return ret, nil
}

// GetMenuById gets the entire menu hierarchy by the menu location id
func (wp *WordPress) GetMenuById(menuId int64) ([]*MenuItem, error) {
	return wp.getMenuItems(&ObjectQueryOptions{MenuId: menuId})
}

// GetMenuBySlug gets the entire menu hierarchy by the menu location slug
func (wp *WordPress) GetMenuBySlug(menuSlug string) ([]*MenuItem, error) {
	return wp.getMenuItems(&ObjectQueryOptions{MenuName: menuSlug})
}

// getMenuItems is the most expensive operation in this package... use sparingly...
func (wp *WordPress) getMenuItems(q *ObjectQueryOptions) ([]*MenuItem, error) {
	cacheKey := "wp_menu_"
	if q.MenuName != "" {
		cacheKey += q.MenuName
	} else {
		cacheKey += "id_" + strconv.FormatInt(q.MenuId, 64)
	}

	if !wp.FlushCache {
		var cacheResult []*MenuItem
		err := wp.cacheGet(cacheKey, &cacheResult)
		if err == nil {
			return cacheResult, nil
		}
	}

	q.PerPage = -1
	q.PostType = PostTypeNavMenuItem

	objectIds, err := wp.QueryObjects(q)
	if err != nil {
		return nil, err
	}

	objects, err := wp.GetObjects(objectIds...)

	if len(objects) == 0 {
		return []*MenuItem{}, nil
	}

	// prepare a slice for the query parameters
	var params []interface{}

	// generate the SQL statement
	stmt := "SELECT post_id, meta_key, meta_value FROM " + wp.table("postmeta") + " WHERE post_id IN (?"
	params = append(params, objects[0].Id)
	for _, obj := range objects {
		stmt += ",?"
		params = append(params, obj.Id)
	}
	stmt += ")"

	rows, err := wp.db.Query(stmt, params...)
	if err != nil {
		return nil, err
	}

	metaMap := make(map[int64]map[string]string)
	for rows.Next() {
		var id int64
		var key string
		var value string

		if err := rows.Scan(&id, &key, &value); err != nil {
			return nil, err
		}

		if _, ok := metaMap[id]; !ok {
			metaMap[id] = make(map[string]string)
		}

		metaMap[id][key] = value
	}

	count := 0
	done := make(chan error)

	menuItems := make(map[int64]*MenuItem)
	for _, obj := range objects {
		meta := metaMap[obj.Id]

		mi := &MenuItem{}
		mi.Id = obj.Id
		mi.Order = obj.MenuOrder
		mi.Title = obj.Title

		if objectId, ok := meta["_menu_item_object_id"]; ok && objectId != "" {
			if i, err := strconv.ParseInt(objectId, 10, 64); err == nil {
				mi.ObjectId = i
			}
		}

		if object, ok := meta["_menu_item_object"]; ok && object != "" {
			mi.Object = object
		}

		if target, ok := meta["_menu_item_target"]; ok && target != "" {
			mi.Target = target
		}

		if enc, ok := meta["_menu_item_classes"]; ok && enc != "" {
			if obj, err := phpserialize.Decode(enc); err == nil {
				// its okay if we don't have the classes
				if classes, ok := obj.(map[interface{}]interface{}); ok {
					for _, class := range classes {
						if class, ok := class.(string); ok {
							mi.Classes += class + " "
						}
					}

					mi.Classes = mi.Classes[:len(mi.Classes)-1]
				}
			}
		}

		if xfn, ok := meta["_menu_item_xfn"]; ok && xfn != "" {
			mi.Xfn = xfn
		}

		if url, ok := meta["_menu_item_url"]; ok && url != "" {
			mi.Link = url
		}

		if objType, ok := meta["_menu_item_type"]; ok && objType != "" {
			mi.Type = MenuItemType(objType)
		}

		if parent, ok := meta["_menu_item_menu_item_parent"]; ok && parent != "" {
			if i, err := strconv.ParseInt(parent, 10, 64); err == nil {
				mi.ParentId = i
			}
		}

		if mi.Type != MenuItemTypeCustom {
			count++

			go func() {
				if mi.Type == MenuItemTypeTaxonomy {
					if mi.Object == "category" {
						cats, err := wp.GetCategories(mi.ObjectId)
						if err != nil {
							done <- err
							return
						}

						mi.Title = cats[0].Name
						mi.Link = cats[0].Link
					}

					done <- nil
				} else if mi.Type == MenuItemTypePost {
					if mi.Object == "page" {
						var url string
						pageId := mi.ObjectId
						for pageId != 0 {
							row := wp.db.QueryRow("SELECT post_title, post_name, post_parent "+
								"FROM "+wp.table("posts")+" WHERE ID = ?", pageId)

							var t, n string
							var parent int64
							if err := row.Scan(&t, &n, &parent); err != nil {
								done <- fmt.Errorf("wordpress: %v", err)
							} else {
								if mi.Title == "" {
									mi.Title = t
								}

								url = "/" + n + url
							}

							pageId = parent
						}

						mi.Link = url

						done <- nil
					} else {
						row := wp.db.QueryRow("SELECT YEAR(post_date), MONTH(post_date), post_title, post_name "+
							"FROM "+wp.table("posts")+" WHERE ID = ?", mi.ObjectId)

						var year, month int
						var title, slug string
						if err := row.Scan(&year, &month, &title, &slug); err != nil {
							done <- fmt.Errorf("Unable to get post url - %v; %v", err, mi)
						} else {
							mi.Title = title
							mi.Link = fmt.Sprintf("/%d/%d/%s", year, month, slug)

							done <- nil
						}
					}
				} else {
					done <- nil
				}
			}()
		}

		menuItems[mi.Id] = mi
	}

	var ret []*MenuItem
	for _, mi := range menuItems {
		if mi.ParentId == 0 {
			ret = append(ret, mi)
			continue
		}

		if menuItems[mi.ParentId].Children == nil {
			menuItems[mi.ParentId].Children = make([]*MenuItem, 0, 1)
		}

		menuItems[mi.ParentId].Children = append(menuItems[mi.ParentId].Children, mi)
	}

	sortMenuItems(ret)

	for count > 0 {
		if err := <-done; err != nil {
			return nil, err
		}

		count--
	}

	go func() {
		_ = wp.cacheSet(cacheKey, &ret)
	}()

	return ret, nil
}

// MenuItemList is used for sorting menu items
type MenuItemList []*MenuItem

// Len is the number of elements in the collection.
func (mis MenuItemList) Len() int {
	return len(mis)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (mis MenuItemList) Less(i, j int) bool {
	return mis[i].Order < mis[j].Order
}

// Swap swaps the elements with indexes i and j.
func (mis MenuItemList) Swap(i, j int) {
	tmp := mis[i]
	mis[i] = mis[j]
	mis[j] = tmp
}

func sortMenuItems(mis []*MenuItem) {
	sort.Sort(MenuItemList(mis))
	for _, mi := range mis {
		if mi.Children != nil && len(mi.Children) > 0 {
			sortMenuItems(mi.Children)
		}
	}
}
