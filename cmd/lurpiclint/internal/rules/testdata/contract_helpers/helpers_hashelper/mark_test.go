package mark

import "testing"

func TestTable_contract_anchor_export(t *testing.T) {
	contracttest.AssertAnchorExport(t, nil, nil)
}
