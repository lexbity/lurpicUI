package mark

import "testing"

type Item struct{}

func TestTable_contract_databound(t *testing.T) {
	contracttest.AssertDataBound[Item](t, nil, nil)
}
