package mark

type AnchorMark struct{}

func (m *AnchorMark) ExportAnchors(ctx interface{}) interface{} {
	return nil
}

func NewAnchorMark() *AnchorMark {
	return &AnchorMark{}
}
