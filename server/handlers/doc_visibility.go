package handlers

import "folio/server/models"

type docVisibilityNode struct {
	ID       uint
	ParentID *uint
	Private  bool
	AuthorID uint
}

type docVisibilityIndex struct {
	nodes     map[uint]docVisibilityNode
	effective map[uint]bool
	visible   map[uint]bool
}

// docVisibility 计算当前查看者能看到的文档集合。
// 私密标记会向下继承；子文档私密不会反向影响父文档。
func (a *API) docVisibility(viewer *models.User) docVisibilityIndex {
	var nodes []docVisibilityNode
	a.DB.Model(&models.Document{}).
		Select("id, parent_id, private, author_id").
		Where("kind = ?", models.KindDoc).
		Find(&nodes)
	index := docVisibilityIndex{
		nodes:     make(map[uint]docVisibilityNode, len(nodes)),
		effective: make(map[uint]bool, len(nodes)),
		visible:   make(map[uint]bool, len(nodes)),
	}
	for _, node := range nodes {
		index.nodes[node.ID] = node
	}
	var privateOf func(uint, map[uint]bool) bool
	privateOf = func(id uint, seen map[uint]bool) bool {
		if value, ok := index.effective[id]; ok {
			return value
		}
		node, ok := index.nodes[id]
		if !ok || seen[id] {
			return false
		}
		seen[id] = true
		value := node.Private
		if !value && node.ParentID != nil {
			value = privateOf(*node.ParentID, seen)
		}
		index.effective[id] = value
		return value
	}
	for id := range index.nodes {
		privateOf(id, map[uint]bool{})
	}

	superAdmin := isSuperAdmin(viewer)
	var allowed func(uint) bool
	allowed = func(id uint) bool {
		if superAdmin {
			return true
		}
		if viewer == nil {
			return false
		}
		cur := id
		seen := map[uint]bool{}
		for {
			node, ok := index.nodes[cur]
			if !ok || seen[cur] {
				break
			}
			seen[cur] = true
			if node.Private && node.AuthorID != viewer.ID {
				return false
			}
			if node.ParentID == nil {
				break
			}
			cur = *node.ParentID
		}
		return true
	}
	for id := range index.nodes {
		index.visible[id] = !index.effective[id] || allowed(id)
	}
	return index
}

func (a *API) hiddenDocIDs(viewer *models.User) []uint {
	index := a.docVisibility(viewer)
	ids := make([]uint, 0)
	for id := range index.nodes {
		if !index.visible[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

func (a *API) canViewDocument(viewer *models.User, doc *models.Document) bool {
	if doc == nil || doc.Kind != models.KindDoc {
		return true
	}
	index := a.docVisibility(viewer)
	if visible, ok := index.visible[doc.ID]; ok {
		return visible
	}
	return !doc.Private
}

func (a *API) docIsPrivate(docID uint) bool {
	index := a.docVisibility(nil)
	return index.effective[docID]
}
