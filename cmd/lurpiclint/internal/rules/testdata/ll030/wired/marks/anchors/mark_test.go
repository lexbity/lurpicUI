package mark

import "testing"

func TestAnchorMark_contract_anchor_export(t *testing.T) {
	contracttest.AssertAnchorExport(t, nil, nil)
}
