package mark

//nolint:LL030 // deliberate opt-out
type Table struct{}

func (t *Table) ExportAnchors(ctx interface{}) interface{} {
	return nil
}
