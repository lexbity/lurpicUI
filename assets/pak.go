package assets

import (
	"encoding/binary"
	"unsafe"
)

// PakCompression identifies the block compression used for one TOC entry.
type PakCompression uint8

const (
	PakCompressionNone PakCompression = iota
	PakCompressionLZ4
	PakCompressionZstd
)

// PakHeader occupies the first 32 bytes of the file.
type PakHeader struct {
	TOCOffset  uint64
	DepsOffset uint64
	Magic      [4]byte
	Version    uint32
	TOCCount   uint32
	DepsCount  uint32
}

// PakTOCEntry is one entry in the table of contents.
type PakTOCEntry struct {
	ID               AssetID
	Offset           uint64
	CompressedSize   uint32
	UncompressedSize uint32
	LODLevel         uint8
	AssetType        uint8
	Compression      uint8
	_                uint8
	DepsOffset       uint32
	DepsCount        uint16
	_                [2]byte
}

var (
	_ [32 - int(unsafe.Sizeof(PakHeader{}))]struct{}
	_ [48 - int(unsafe.Sizeof(PakTOCEntry{}))]struct{}
)

// tocSearch performs binary search over a TOC sorted by the first eight bytes of AssetID.
// The returned index is any exact match within the prefix-equivalent cluster.
func tocSearch(toc []PakTOCEntry, id AssetID) (int, bool) {
	key := binary.BigEndian.Uint64(id[:8])
	lo, hi := 0, len(toc)

	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		midKey := binary.BigEndian.Uint64(toc[mid].ID[:8])
		switch {
		case midKey < key:
			lo = mid + 1
		case midKey > key:
			hi = mid
		default:
			left := mid
			for left > lo && binary.BigEndian.Uint64(toc[left-1].ID[:8]) == key {
				left--
			}
			for i := left; i < hi && binary.BigEndian.Uint64(toc[i].ID[:8]) == key; i++ {
				if toc[i].ID == id {
					return i, true
				}
			}
			return 0, false
		}
	}

	return 0, false
}
