package assets

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// AssetID is a stable 128-bit identifier.
type AssetID [16]byte

// String returns the standard hyphenated UUID representation.
func (id AssetID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], id[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], id[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], id[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], id[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], id[10:16])
	return string(buf[:])
}

// IsZero reports whether the identifier is all zero bytes.
func (id AssetID) IsZero() bool {
	return id == AssetID{}
}

// ParseAssetID parses a hyphenated or 32-character hex UUID string.
func ParseAssetID(s string) (AssetID, error) {
	var id AssetID
	cleaned := strings.ReplaceAll(strings.TrimSpace(s), "-", "")
	if len(cleaned) != 32 {
		return id, fmt.Errorf("invalid asset id %q", s)
	}
	raw, err := hex.DecodeString(cleaned)
	if err != nil {
		return id, fmt.Errorf("invalid asset id %q: %w", s, err)
	}
	copy(id[:], raw)
	return id, nil
}

// MarshalText implements encoding.TextMarshaler.
func (id AssetID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (id *AssetID) UnmarshalText(text []byte) error {
	parsed, err := ParseAssetID(string(text))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// MarshalJSON encodes the asset ID as a quoted UUID string.
func (id AssetID) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", id.String())), nil
}

// UnmarshalJSON decodes the asset ID from a quoted UUID string.
func (id *AssetID) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty asset id json")
	}
	if string(data) == "null" {
		*id = AssetID{}
		return nil
	}
	var text string
	if err := jsonUnmarshalString(data, &text); err != nil {
		return err
	}
	parsed, err := ParseAssetID(text)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// AssetType identifies a high-level asset category used by the cook/runtime pipeline.
type AssetType uint8

const (
	AssetTypeSVG AssetType = iota
	AssetTypeImage
	AssetTypeFont
	AssetTypeConfig
)

// AssetTypeTexture is an alias for material textures, which use the image compiler path.
const AssetTypeTexture = AssetTypeImage

func (t AssetType) String() string {
	switch t {
	case AssetTypeSVG:
		return "svg"
	case AssetTypeImage:
		return "image"
	case AssetTypeFont:
		return "font"
	case AssetTypeConfig:
		return "config"
	default:
		return fmt.Sprintf("asset-type(%d)", t)
	}
}

func jsonUnmarshalString(data []byte, dst *string) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*dst = s
	return nil
}
