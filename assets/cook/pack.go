package cook

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"

	"codeburg.org/lexbit/lurpicui/assets"
)

const pakVersion = 2

// Packer writes a .pak archive from a resolved dependency tree.
type Packer struct {
	Manifest *Manifest
}

type packBlock struct {
	entry assets.PakTOCEntry
	data  []byte
}

type depSpan struct {
	offset uint32
	count  uint16
}

// Pack serializes tree into a complete pak archive.
func (p *Packer) Pack(tree *DependencyTree) ([]byte, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil dependency tree")
	}

	depsTable, depSpans := buildDepsTable(tree)
	blocks, err := p.buildBlocks(tree, depSpans)
	if err != nil {
		return nil, err
	}

	sort.Slice(blocks, func(i, j int) bool {
		return compareTOCEntry(blocks[i].entry, blocks[j].entry) < 0
	})

	const headerSize = 32
	const tocEntrySize = 48
	const depEntrySize = 16

	tocOffset := uint64(headerSize)
	tocSize := uint64(len(blocks) * tocEntrySize)
	depsOffset := alignUp(tocOffset+tocSize, 8)
	depsSize := uint64(len(depsTable) * depEntrySize)
	dataOffset := alignUp(depsOffset+depsSize, 8)

	current := dataOffset
	for i := range blocks {
		blocks[i].entry.Offset = current
		current = alignUp(current+uint64(len(blocks[i].data)), 8)
	}

	buf := make([]byte, current)
	writePakHeader(buf[:headerSize], assets.PakHeader{
		Magic:      [4]byte{'L', 'U', 'R', 'P'},
		Version:    pakVersion,
		TOCOffset:  tocOffset,
		TOCCount:   uint32(len(blocks)), //nolint:gosec // integer overflow conversion
		DepsOffset: depsOffset,
		DepsCount:  uint32(len(depsTable)), //nolint:gosec // integer overflow conversion
	})

	for i, blk := range blocks {
		writePakTOCEntry(buf[int(tocOffset)+i*tocEntrySize:], blk.entry)
		copy(buf[int(blk.entry.Offset):int(blk.entry.Offset)+len(blk.data)], blk.data) //nolint:gosec // integer overflow conversion
	}
	for i, id := range depsTable {
		copy(buf[int(depsOffset)+i*depEntrySize:], id[:]) //nolint:gosec // integer overflow conversion
	}

	return buf, nil
}

func (p *Packer) buildBlocks(tree *DependencyTree, depSpans map[assets.AssetID]depSpan) ([]packBlock, error) {
	assetsList := make([]AssetNode, 0, len(tree.Leaves)+len(tree.Configs))
	assetsList = append(assetsList, tree.Leaves...)
	for _, cfg := range tree.Configs {
		assetsList = append(assetsList, cfg.AssetNode)
	}

	blocks := make([]packBlock, 0)
	for _, node := range assetsList {
		if len(node.LODs) == 0 {
			return nil, fmt.Errorf("asset %q has no LODs", node.Path)
		}
		for _, lod := range node.LODs {
			compression, err := p.compressionFor(node.Type, lod.Level)
			if err != nil {
				return nil, err
			}
			data, err := compressBlock(compression, lod.Data)
			if err != nil {
				return nil, fmt.Errorf("compress %q lod %d: %w", node.Path, lod.Level, err)
			}
			entry := assets.PakTOCEntry{
				ID:               node.ID,
				CompressedSize:   uint32(len(data)),     //nolint:gosec // integer overflow conversion
				UncompressedSize: uint32(len(lod.Data)), //nolint:gosec // integer overflow conversion
				LODLevel:         uint8(lod.Level),      //nolint:gosec // integer overflow conversion
				AssetType:        uint8(node.Type),
				Compression:      uint8(compression),
			}
			if span, ok := depSpans[node.ID]; ok {
				entry.DepsOffset = span.offset
				entry.DepsCount = span.count
			}
			blocks = append(blocks, packBlock{entry: entry, data: data})
		}
	}
	return blocks, nil
}

func buildDepsTable(tree *DependencyTree) ([]assets.AssetID, map[assets.AssetID]depSpan) {
	depsTable := make([]assets.AssetID, 0)
	spans := make(map[assets.AssetID]depSpan, len(tree.Configs))
	for _, cfg := range tree.Configs {
		start := len(depsTable)
		depsTable = append(depsTable, cfg.Deps...)
		spans[cfg.ID] = depSpan{
			offset: uint32(start),         //nolint:gosec // integer overflow conversion
			count:  uint16(len(cfg.Deps)), //nolint:gosec // integer overflow conversion
		}
	}
	return depsTable, spans
}

func (p *Packer) compressionFor(assetType assets.AssetType, lodLevel int) (assets.PakCompression, error) {
	switch assetType {
	case assets.AssetTypeSVG:
		if lodLevel == 0 {
			return parsePakCompressionOrDefault(p.manifestPackSVG(), assets.PakCompressionLZ4)
		}
		return assets.PakCompressionNone, nil
	case assets.AssetTypeImage:
		return parsePakCompressionOrDefault(p.manifestPackImage(), assets.PakCompressionNone)
	case assets.AssetTypeFont:
		return parsePakCompressionOrDefault(p.manifestPackFont(), assets.PakCompressionZstd)
	case assets.AssetTypeConfig:
		return parsePakCompressionOrDefault(p.manifestPackConfig(), assets.PakCompressionZstd)
	default:
		return assets.PakCompressionNone, nil
	}
}

func (p *Packer) manifestPackSVG() string {
	if p == nil || p.Manifest == nil {
		return ""
	}
	return p.Manifest.Pack.SVGCompression
}

func (p *Packer) manifestPackImage() string {
	if p == nil || p.Manifest == nil {
		return ""
	}
	return p.Manifest.Pack.ImageCompression
}

func (p *Packer) manifestPackFont() string {
	if p == nil || p.Manifest == nil {
		return ""
	}
	return p.Manifest.Pack.FontCompression
}

func (p *Packer) manifestPackConfig() string {
	if p == nil || p.Manifest == nil {
		return ""
	}
	return p.Manifest.Pack.ConfigCompression
}

func parsePakCompressionOrDefault(spec string, def assets.PakCompression) (assets.PakCompression, error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" {
		return def, nil
	}
	switch spec {
	case "none":
		return assets.PakCompressionNone, nil
	case "lz4":
		return assets.PakCompressionLZ4, nil
	case "zstd":
		return assets.PakCompressionZstd, nil
	default:
		return 0, fmt.Errorf("unknown compression %q", spec)
	}
}

func compressBlock(kind assets.PakCompression, src []byte) ([]byte, error) {
	switch kind {
	case assets.PakCompressionNone:
		return append([]byte(nil), src...), nil
	case assets.PakCompressionLZ4:
		dst := make([]byte, lz4.CompressBlockBound(len(src)))
		n, err := lz4.CompressBlock(src, dst, nil)
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), dst[:n]...), nil
	case assets.PakCompressionZstd:
		return append([]byte(nil), zstd.EncodeTo(nil, src)...), nil
	default:
		return nil, fmt.Errorf("unsupported compression kind %d", kind)
	}
}

func compareTOCEntry(a, b assets.PakTOCEntry) int {
	ak := binary.BigEndian.Uint64(a.ID[:8])
	bk := binary.BigEndian.Uint64(b.ID[:8])
	switch {
	case ak < bk:
		return -1
	case ak > bk:
		return 1
	}
	switch {
	case a.LODLevel < b.LODLevel:
		return -1
	case a.LODLevel > b.LODLevel:
		return 1
	}
	return bytes.Compare(a.ID[:], b.ID[:])
}

func alignUp(v uint64, align uint64) uint64 {
	if align == 0 {
		return v
	}
	rem := v % align
	if rem == 0 {
		return v
	}
	return v + align - rem
}

func writePakHeader(dst []byte, header assets.PakHeader) {
	if len(dst) < 32 {
		return
	}
	copy(dst[0:4], header.Magic[:])
	binary.LittleEndian.PutUint32(dst[4:8], header.Version)
	binary.LittleEndian.PutUint64(dst[8:16], header.TOCOffset)
	binary.LittleEndian.PutUint32(dst[16:20], header.TOCCount)
	binary.LittleEndian.PutUint64(dst[20:28], header.DepsOffset)
	binary.LittleEndian.PutUint32(dst[28:32], header.DepsCount)
}

// PakContentHash computes the SHA-256 of a packed archive for use in the
// extraction sidecar (assets.pak.meta). The build tool calls this after Pack
// and writes the result alongside the .pak file.
func PakContentHash(pakData []byte) [32]byte {
	return sha256.Sum256(pakData)
}

// WritePak writes the pak archive and its sidecar metadata to destDir.
// The sidecar (assets.pak.meta) carries the content hash so the runtime's
// extraction gate can decide whether to re-extract without hashing the
// entire file. Each call produces:
//
//	<destDir>/assets.pak
//	<destDir>/assets.pak.meta
func WritePak(destDir string, pakData []byte) error {
	//nolint:gosec // build output dir
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("write pak mkdir: %w", err)
	}
	pakPath := filepath.Join(destDir, "assets.pak")
	//nolint:gosec // shared build artifact
	if err := os.WriteFile(pakPath, pakData, 0o644); err != nil {
		return fmt.Errorf("write pak: %w", err)
	}
	meta := newPakSidecar(PakContentHash(pakData))
	metaPath := filepath.Join(destDir, "assets.pak.meta")
	f, err := os.Create(metaPath) //nolint:gosec // path from user config
	if err != nil {
		return fmt.Errorf("create meta: %w", err)
	}
	if err := json.NewEncoder(f).Encode(meta); err != nil {
		f.Close()
		_ = os.Remove(metaPath)
		return fmt.Errorf("encode meta: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(metaPath)
		return fmt.Errorf("close meta: %w", err)
	}
	return nil
}

// pakSidecar mirrors assets.PakMeta for JSON encoding without a circular dep.
type pakSidecar struct {
	Version int    `json:"v"`
	Hash    string `json:"bh"`
}

func newPakSidecar(hash [32]byte) pakSidecar {
	return pakSidecar{
		Version: 1,
		Hash:    hex.EncodeToString(hash[:]),
	}
}

func writePakTOCEntry(dst []byte, entry assets.PakTOCEntry) {
	if len(dst) < 48 {
		return
	}
	copy(dst[0:16], entry.ID[:])
	binary.LittleEndian.PutUint64(dst[16:24], entry.Offset)
	binary.LittleEndian.PutUint32(dst[24:28], entry.CompressedSize)
	binary.LittleEndian.PutUint32(dst[28:32], entry.UncompressedSize)
	dst[32] = entry.LODLevel
	dst[33] = entry.AssetType
	dst[34] = entry.Compression
	dst[35] = 0
	binary.LittleEndian.PutUint32(dst[36:40], entry.DepsOffset)
	binary.LittleEndian.PutUint16(dst[40:42], entry.DepsCount)
	dst[42] = 0
	dst[43] = 0
	dst[44] = 0
	dst[45] = 0
	dst[46] = 0
	dst[47] = 0
}
