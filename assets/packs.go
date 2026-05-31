package assets

import (
	"fmt"
	"sync"
)

// DeliveryType describes when an asset pack is delivered to the device.
type DeliveryType int

const (
	DeliveryInstallTime DeliveryType = iota // Available immediately after install.
	DeliveryFastFollow                      // Downloaded automatically after install.
	DeliveryOnDemand                        // Downloaded when first requested.
)

// PackDescriptor describes one named asset pack.
type PackDescriptor struct {
	Name     string
	Delivery DeliveryType
	Source   AssetSource // the pack's AssetSource (e.g., a PakFS)
}

// PackAssetSource is an AssetSource that resolves reads across multiple named
// asset packs. It checks packs in order: install-time first, then fast-follow,
// then on-demand. For on-demand packs that are not yet downloaded, it returns
// a not-found error (the caller retries after the download completes).
type PackAssetSource struct {
	mu    sync.RWMutex
	packs []PackDescriptor
}

// NewPackAssetSource creates a multi-pack AssetSource from the given descriptors.
func NewPackAssetSource(packs []PackDescriptor) *PackAssetSource {
	return &PackAssetSource{packs: packs}
}

// ReadLOD implements AssetSource by searching packs in delivery-type order
// (install-time first, then fast-follow, then on-demand).
func (s *PackAssetSource) ReadLOD(id AssetID, lod int) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("pack source: nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.packs {
		if p.Source == nil {
			continue
		}
		data, err := p.Source.ReadLOD(id, lod)
		if err == nil {
			return data, nil
		}
		// On-demand packs that haven't been downloaded return ErrNotExist.
		// The caller should trigger the download and retry.
	}
	return nil, fmt.Errorf("pack source: asset %s lod %d not found in any pack", id.String(), lod)
}

// AddPack appends a new pack. Used for runtime-registered on-demand packs
// after their download completes.
func (s *PackAssetSource) AddPack(pack PackDescriptor) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packs = append(s.packs, pack)
}
