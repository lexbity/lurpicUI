package ll013_bad

type BadFacet struct {
	facet.Facet
	cachedTokens theme.Tokens
}

func (f *BadFacet) OnAttach() {
	f.cachedTokens = theme.DefaultTokens()
}
