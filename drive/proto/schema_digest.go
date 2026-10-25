package proto

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// schemaDigest computes SchemaDigest (see registry.go). The input is the
// canonical registry description, not any JSON byte form, so reordering
// struct fields cannot change the digest while a changed method set always
// does.
func schemaDigest() string {
	names := AdvertisedMethodNames()
	h := sha256.New()
	h.Write([]byte("ldp/2\x00"))
	h.Write([]byte(strings.Join(names, "\x00")))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
