package cook

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"

	"codeburg.org/lexbit/lurpicui/assets"
)

func TestPackerPackRoundTrip(t *testing.T) {
	tree := &DependencyTree{
		Leaves: []AssetNode{
			{
				ID:   makeTestAssetID(0x1122334455667788, 0x01),
				Path: "assets/icons/a.svg",
				Type: assets.AssetTypeSVG,
				LODs: []CompiledLOD{
					{Level: 0, Data: bytes.Repeat([]byte("svg-lod0:"), 24)},
					{Level: 1, Data: bytes.Repeat([]byte{0x10, 0x20, 0x30, 0x40}, 32)},
					{Level: 2, Data: []byte{0xaa, 0xbb, 0xcc, 0xdd}},
				},
			},
			{
				ID:   makeTestAssetID(0x1122334455667788, 0x02),
				Path: "assets/fonts/a.ttf",
				Type: assets.AssetTypeFont,
				LODs: []CompiledLOD{
					{Level: 0, Data: bytes.Repeat([]byte("font-lod0:"), 18)},
					{Level: 1, Data: bytes.Repeat([]byte("font-lod1:"), 12)},
				},
			},
		},
		Configs: []ConfigNode{
			{
				AssetNode: AssetNode{
					ID:   makeTestAssetID(0x99aabbccddeeff00, 0x03),
					Path: "assets/config/theme.toml",
					Type: assets.AssetTypeConfig,
					LODs: []CompiledLOD{{Level: 0, Data: bytes.Repeat([]byte("config-lod0:"), 10)}},
				},
				Deps: []assets.AssetID{
					makeTestAssetID(0x1122334455667788, 0x01),
					makeTestAssetID(0x1122334455667788, 0x02),
				},
			},
		},
	}

	packer := &Packer{}
	pak, err := packer.Pack(tree)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if len(pak) == 0 {
		t.Fatal("expected pak output")
	}

	header := decodePakHeader(t, pak[:32])
	if string(header.Magic[:]) != "LURP" {
		t.Fatalf("unexpected magic: %q", header.Magic)
	}
	if header.Version != pakVersion {
		t.Fatalf("unexpected version: %d", header.Version)
	}
	if header.TOCOffset != 32 {
		t.Fatalf("unexpected toc offset: %d", header.TOCOffset)
	}
	if header.TOCCount != 6 {
		t.Fatalf("unexpected toc count: %d", header.TOCCount)
	}
	if header.DepsCount != 2 {
		t.Fatalf("unexpected deps count: %d", header.DepsCount)
	}

	tocStart := int(header.TOCOffset)
	tocEnd := tocStart + int(header.TOCCount)*48
	toc := make([]assets.PakTOCEntry, 0, header.TOCCount)
	for i := 0; i < int(header.TOCCount); i++ {
		entry := decodePakTOCEntry(t, pak[tocStart+i*48:tocStart+(i+1)*48])
		toc = append(toc, entry)
	}
	if !isSortedTOC(toc) {
		t.Fatalf("toc not sorted: %+v", toc)
	}

	depsStart := int(header.DepsOffset)
	depsEnd := depsStart + int(header.DepsCount)*16
	if depsStart < tocEnd {
		t.Fatalf("deps overlap toc: deps=%d tocEnd=%d", depsStart, tocEnd)
	}

	expectedDeps := []assets.AssetID{
		makeTestAssetID(0x1122334455667788, 0x01),
		makeTestAssetID(0x1122334455667788, 0x02),
	}
	for i, want := range expectedDeps {
		var got assets.AssetID
		copy(got[:], pak[depsStart+i*16:depsStart+(i+1)*16])
		if got != want {
			t.Fatalf("unexpected dep %d: got %s want %s", i, got, want)
		}
	}

	dataStart := alignUp(uint64(depsEnd), 8)
	if int(dataStart) > len(pak) {
		t.Fatalf("invalid data start: %d > %d", dataStart, len(pak))
	}

	expectedPayloads := map[string][]byte{
		keyFor(makeTestAssetID(0x1122334455667788, 0x01), 0): bytes.Repeat([]byte("svg-lod0:"), 24),
		keyFor(makeTestAssetID(0x1122334455667788, 0x01), 1): bytes.Repeat([]byte{0x10, 0x20, 0x30, 0x40}, 32),
		keyFor(makeTestAssetID(0x1122334455667788, 0x01), 2): []byte{0xaa, 0xbb, 0xcc, 0xdd},
		keyFor(makeTestAssetID(0x1122334455667788, 0x02), 0): bytes.Repeat([]byte("font-lod0:"), 18),
		keyFor(makeTestAssetID(0x1122334455667788, 0x02), 1): bytes.Repeat([]byte("font-lod1:"), 12),
		keyFor(makeTestAssetID(0x99aabbccddeeff00, 0x03), 0): bytes.Repeat([]byte("config-lod0:"), 10),
	}

	for _, entry := range toc {
		want := expectedPayloads[keyFor(entry.ID, int(entry.LODLevel))]
		if want == nil {
			t.Fatalf("unexpected toc entry: %+v", entry)
		}
		block := pak[int(entry.Offset) : int(entry.Offset)+int(entry.CompressedSize)]
		got, err := decodePakBlock(entry.Compression, block, int(entry.UncompressedSize))
		if err != nil {
			t.Fatalf("decode block id=%s lod=%d: %v", entry.ID, entry.LODLevel, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("payload mismatch id=%s lod=%d", entry.ID, entry.LODLevel)
		}
	}

	// Spot-check the exact compression selection.
	if entry, ok := findTOCEntry(toc, makeTestAssetID(0x1122334455667788, 0x01), 0); !ok || entry.Compression != uint8(assets.PakCompressionLZ4) {
		t.Fatalf("expected svg lod0 lz4 entry, got %+v", entry)
	}
	if entry, ok := findTOCEntry(toc, makeTestAssetID(0x1122334455667788, 0x01), 1); !ok || entry.Compression != uint8(assets.PakCompressionNone) {
		t.Fatalf("expected svg lod1 none entry, got %+v", entry)
	}
	if entry, ok := findTOCEntry(toc, makeTestAssetID(0x1122334455667788, 0x02), 0); !ok || entry.Compression != uint8(assets.PakCompressionZstd) {
		t.Fatalf("expected font zstd entry, got %+v", entry)
	}
	if entry, ok := findTOCEntry(toc, makeTestAssetID(0x99aabbccddeeff00, 0x03), 0); !ok || entry.DepsOffset != 0 || entry.DepsCount != 2 {
		t.Fatalf("expected config deps entry, got %+v", entry)
	}

	idx, ok := searchTOCEntry(toc, makeTestAssetID(0x1122334455667788, 0x02))
	if !ok {
		t.Fatal("expected tocSearch hit for font asset")
	}
	if toc[idx].LODLevel != 0 {
		t.Fatalf("unexpected tocSearch result: %+v", toc[idx])
	}
}

func decodePakHeader(t *testing.T, data []byte) assets.PakHeader {
	t.Helper()
	if len(data) < 32 {
		t.Fatalf("short header: %d", len(data))
	}
	var header assets.PakHeader
	copy(header.Magic[:], data[0:4])
	header.Version = binary.LittleEndian.Uint32(data[4:8])
	header.TOCOffset = binary.LittleEndian.Uint64(data[8:16])
	header.TOCCount = binary.LittleEndian.Uint32(data[16:20])
	header.DepsOffset = binary.LittleEndian.Uint64(data[20:28])
	header.DepsCount = binary.LittleEndian.Uint32(data[28:32])
	return header
}

func decodePakTOCEntry(t *testing.T, data []byte) assets.PakTOCEntry {
	t.Helper()
	if len(data) < 48 {
		t.Fatalf("short toc entry: %d", len(data))
	}
	var entry assets.PakTOCEntry
	copy(entry.ID[:], data[0:16])
	entry.Offset = binary.LittleEndian.Uint64(data[16:24])
	entry.CompressedSize = binary.LittleEndian.Uint32(data[24:28])
	entry.UncompressedSize = binary.LittleEndian.Uint32(data[28:32])
	entry.LODLevel = data[32]
	entry.AssetType = data[33]
	entry.Compression = data[34]
	entry.DepsOffset = binary.LittleEndian.Uint32(data[36:40])
	entry.DepsCount = binary.LittleEndian.Uint16(data[40:42])
	return entry
}

func decodePakBlock(kind uint8, data []byte, uncompressedSize int) ([]byte, error) {
	switch assets.PakCompression(kind) {
	case assets.PakCompressionNone:
		return append([]byte(nil), data...), nil
	case assets.PakCompressionLZ4:
		out := make([]byte, uncompressedSize)
		n, err := lz4.UncompressBlock(data, out)
		if err != nil {
			return nil, err
		}
		return out[:n], nil
	case assets.PakCompressionZstd:
		return zstd.DecodeTo(nil, data)
	default:
		return nil, nil
	}
}

func findTOCEntry(toc []assets.PakTOCEntry, id assets.AssetID, lod int) (assets.PakTOCEntry, bool) {
	for _, entry := range toc {
		if entry.ID == id && int(entry.LODLevel) == lod {
			return entry, true
		}
	}
	return assets.PakTOCEntry{}, false
}

func isSortedTOC(toc []assets.PakTOCEntry) bool {
	for i := 1; i < len(toc); i++ {
		if compareTOCEntry(toc[i-1], toc[i]) > 0 {
			return false
		}
	}
	return true
}

func searchTOCEntry(toc []assets.PakTOCEntry, id assets.AssetID) (int, bool) {
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

func makeTestAssetID(prefix uint64, suffix byte) assets.AssetID {
	var id assets.AssetID
	binary.BigEndian.PutUint64(id[:8], prefix)
	id[15] = suffix
	return id
}

func keyFor(id assets.AssetID, lod int) string {
	return id.String() + ":" + string(rune('0'+lod))
}
