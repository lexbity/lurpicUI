package drive

import "bytes"

// bytesReader wraps bytes.NewReader (named so the ledger file stays
// focused on ledger policy).
func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
