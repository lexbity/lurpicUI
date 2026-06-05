package assets

// ReadGate wraps an AssetSource with blocking synchronization so tests
// can provably interleave a Close with an in-flight read. Use it for
// deterministic close-during-read assertions (see assets M4).
type ReadGate struct {
	source  AssetSource
	Started chan struct{}
	Release chan struct{}
}

// NewReadGate creates a ReadGate that wraps source.
func NewReadGate(source AssetSource) *ReadGate {
	return &ReadGate{
		source:  source,
		Started: make(chan struct{}),
		Release: make(chan struct{}),
	}
}

// ReadLOD blocks until Release is closed, then delegates to the wrapped source.
func (g *ReadGate) ReadLOD(id AssetID, lod int) ([]byte, error) {
	close(g.Started)
	<-g.Release
	return g.source.ReadLOD(id, lod)
}
