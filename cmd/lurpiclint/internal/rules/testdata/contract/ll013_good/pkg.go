package ll013_good

type GoodFacet struct {
	facet.Facet
}

func (f *GoodFacet) OnProject(ctx facet.ProjectionContext) {
	tokens := ctx.ThemeTokens()
	_ = tokens
}
