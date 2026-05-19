package svg

func isDefinitionOnlyNode(name string) bool {
	switch name {
	case "clipPath", "linearGradient":
		return true
	default:
		return false
	}
}

func isShapeNode(name string) bool {
	switch name {
	case "path", "rect", "circle", "ellipse", "line", "polyline", "polygon":
		return true
	default:
		return false
	}
}
