package mark

import "testing"

func TestFocusMark_contract_focusable(t *testing.T) {
	contracttest.AssertFocusable(t, nil)
}
