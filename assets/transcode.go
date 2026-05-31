package assets

import (
	"encoding/binary"
	"fmt"
)

// KTX2 identifier bytes at offset 0.
var ktx2Identifier = []byte{0xAB, 0x4B, 0x54, 0x58, 0x20, 0x32, 0x30, 0xBB, 0x0D, 0x0A, 0x1A, 0x0A}

// ktx2HeaderLen is the fixed header size before the DFD.
const ktx2HeaderLen = 80

// decodeImageHeader parses the minimum header from a KTX2 byte slice to
// extract pixel width and height. Returns an error if the data is not
// valid KTX2 or is too short.
func decodeImageHeader(data []byte) (width int32, height int32, err error) {
	if len(data) < ktx2HeaderLen {
		return 0, 0, fmt.Errorf("ktx2: data too short (%d bytes, need %d)", len(data), ktx2HeaderLen)
	}
	for i, b := range ktx2Identifier {
		if data[i] != b {
			return 0, 0, fmt.Errorf("ktx2: invalid identifier byte at %d", i)
		}
	}
	w := binary.LittleEndian.Uint32(data[20:24])
	h := binary.LittleEndian.Uint32(data[24:28])
	if w == 0 {
		return 0, 0, fmt.Errorf("ktx2: zero pixel width")
	}
	return int32(w), int32(h), nil
}

// compressedSizeForTarget returns the equivalent GPU memory footprint in
// bytes for an image of the given dimensions when transcoded to the target
// format. This is an estimate used for GPU budget accounting, not an exact
// transcode result.
func compressedSizeForTarget(width, height int32, format uint32) int64 {
	if width <= 0 || height <= 0 {
		return 0
	}
	w := int64(width)
	h := int64(height)

	// Bytes per pixel for each format.
	var bpp float64
	switch format {
	case 1: // TextureFormatASTC4x4
		bpp = 8.0 / 16.0 // 8 bytes per 4x4 block
	case 2: // TextureFormatBC7
		bpp = 16.0 / 16.0 // 16 bytes per 4x4 block
	default: // TextureFormatRGBA8
		return w * h * 4
	}

	// Round up dimensions to block boundary for block-compressed formats.
	blockW := (w + 3) / 4 * 4
	blockH := (h + 3) / 4 * 4
	bytes := int64(float64(blockW*blockH) * bpp)
	if bytes < 1 {
		return 1
	}
	return bytes
}
