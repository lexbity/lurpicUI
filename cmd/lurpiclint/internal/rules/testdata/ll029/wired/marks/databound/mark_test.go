package mark

import "testing"

type item struct{}

func TestDataMark_contract_databound(t *testing.T) {
	contracttest.AssertDataBound[item](t, nil, nil, nil)
}
