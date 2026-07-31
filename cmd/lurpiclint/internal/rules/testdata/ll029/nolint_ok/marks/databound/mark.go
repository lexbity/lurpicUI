package mark

//nolint:LL029 // deliberate opt-out
type DataMark struct{}

func (m *DataMark) BoundData() any {
	return nil
}

func NewDataMark() *DataMark {
	return &DataMark{}
}
