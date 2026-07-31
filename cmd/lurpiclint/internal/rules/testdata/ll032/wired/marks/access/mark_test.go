package mark

import "testing"

func TestAccessMark_contract_accessible(t *testing.T) {
	contracttest.AssertAccessible(t, nil, "group")
}
