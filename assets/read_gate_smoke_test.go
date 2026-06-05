package assets

import (
	"errors"
	"testing"
)

type gateTestSource struct {
	data map[[2]uint64][]byte
}

func (s gateTestSource) ReadLOD(id AssetID, lod int) ([]byte, error) {
	if s.data == nil {
		return nil, errors.New("gateTestSource: no data")
	}
	data, ok := s.data[[2]uint64{assetIDKey(id), uint64(lod)}]
	if !ok {
		return nil, errors.New("gateTestSource: not found")
	}
	return append([]byte(nil), data...), nil
}

func TestReadGate_blocks_until_released(t *testing.T) {
	id := AssetID{1}
	source := gateTestSource{
		data: map[[2]uint64][]byte{
			{assetIDKey(id), 0}: []byte("hello"),
		},
	}
	g := NewReadGate(source)

	resultCh := make(chan readGateResult, 1)
	go func() {
		data, err := g.ReadLOD(id, 0)
		resultCh <- readGateResult{data: data, err: err}
	}()

	<-g.Started

	select {
	case <-resultCh:
		t.Fatal("read completed before release")
	default:
	}

	close(g.Release)

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("ReadLOD: %v", result.err)
	}
	if string(result.data) != "hello" {
		t.Fatalf("got %q, want %q", string(result.data), "hello")
	}
}

func TestReadGate_started_signals_in_flight(t *testing.T) {
	g := NewReadGate(gateTestSource{})

	go func() {
		g.ReadLOD(AssetID{1}, 0)
	}()

	<-g.Started
	close(g.Release)
}

type readGateResult struct {
	data []byte
	err  error
}
