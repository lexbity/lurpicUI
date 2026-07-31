package mark

type DataMark struct{}

func (m *DataMark) BoundData() any {
	return nil
}

func NewDataMark() *DataMark {
	return &DataMark{}
}
