package assets

import "codeburg.org/lexbit/lurpicui/signal"

// AssetSignalSet groups the ready and invalidation signals for one asset.
type AssetSignalSet struct {
	Ready       signal.Signal[AssetReadySignal]
	Invalidated signal.Signal[AssetInvalidatedSignal]
}

func newAssetSignalSet(id AssetID) *AssetSignalSet {
	return &AssetSignalSet{
		Ready:       signal.NewSignal[AssetReadySignal]("assets.ready:" + id.String()),
		Invalidated: signal.NewSignal[AssetInvalidatedSignal]("assets.invalidated:" + id.String()),
	}
}
