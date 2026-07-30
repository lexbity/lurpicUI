package ll024_good_tree

import (
	"codeburg.org/lexbit/lurpicui/store"
)

type TreeNode struct {
	ID   string
	Data string
}

type TreeNavigator struct {
	Data *store.ValueStore[[]TreeNode]
}

// NewTreeNavigator has no store param — it owns its store.  No fire.
func NewTreeNavigator(nodes []TreeNode) *TreeNavigator {
	return &TreeNavigator{Data: store.NewValueStore(nodes)}
}
