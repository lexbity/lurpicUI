package mark

type Table struct{}

func (t *Table) ExportAnchors(ctx interface{}) interface{} {
	return nil
}

type Other struct{}

func (o *Other) BoundData() interface{} {
	return nil
}
