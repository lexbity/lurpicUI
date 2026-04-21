package uinav

import uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
import "codeburg.org/lexbit/lurpicui/marks/structure"

type TabsVariant = uirecipe.TabsVariant

const (
	TabsStandard = uirecipe.TabsStandard
	TabsCompact  = uirecipe.TabsCompact
)

type ScrollbarOrientation uint8

const (
	ScrollbarHorizontal ScrollbarOrientation = iota
	ScrollbarVertical
)

type MenuVariant = uirecipe.MenuVariant

const (
	MenuStandard = uirecipe.MenuStandard
	MenuDense    = uirecipe.MenuDense
)

type AnchorSourceRef = structure.AnchorSourceRef
