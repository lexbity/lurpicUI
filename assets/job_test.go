package assets

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

type staticSource struct {
	data map[[2]uint64][]byte
}

func (s staticSource) ReadLOD(id AssetID, lod int) ([]byte, error) {
	return append([]byte(nil), s.data[[2]uint64{assetIDKey(id), uint64(lod)}]...), nil
}

type blockingSource struct {
	payload []byte
	started chan struct{}
	release chan struct{}
}

type captureScheduler struct {
	jobs chan *AssetLoadJob
}

func (s captureScheduler) Schedule(job *AssetLoadJob) error {
	s.jobs <- job
	return nil
}

func (s *blockingSource) ReadLOD(id AssetID, lod int) ([]byte, error) {
	if s.started != nil {
		close(s.started)
		s.started = nil
	}
	<-s.release
	return append([]byte(nil), s.payload...), nil
}

func assetIDKey(id AssetID) uint64 {
	return binary.LittleEndian.Uint64(id[:8]) ^ binary.LittleEndian.Uint64(id[8:])
}

func TestAssetLoadJobExecuteDecodesPayloads(t *testing.T) {
	t.Run("svg lod0 lz4", func(t *testing.T) {
		raw := []byte("geometry-bytes")
		dst := make([]byte, lz4.CompressBlockBound(len(raw)))
		n, err := lz4.CompressBlock(raw, dst, nil)
		if err != nil {
			t.Fatalf("compress: %v", err)
		}
		job := &AssetLoadJob{
			ID:      mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdef"),
			Type:    AssetTypeSVG,
			LOD:     0,
			Source:  staticSource{data: map[[2]uint64][]byte{{assetIDKey(mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdef")), 0}: append([]byte(nil), dst[:n]...)}},
			Backend: BackendSoftware,
		}
		job.Execute()
		if job.Err != nil {
			t.Fatalf("execute: %v", job.Err)
		}
		got, ok := job.Result.(*DecodedSVGLOD0)
		if !ok {
			t.Fatalf("unexpected result type: %T", job.Result)
		}
		if !bytes.Equal(got.Data, raw) {
			t.Fatalf("unexpected payload: %q", got.Data)
		}
	})

	t.Run("font lod0 zstd", func(t *testing.T) {
		raw := []byte("font-bytes")
		enc := zstd.EncodeTo(nil, raw)
		job := &AssetLoadJob{
			ID:      mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdee"),
			Type:    AssetTypeFont,
			LOD:     0,
			Source:  staticSource{data: map[[2]uint64][]byte{{assetIDKey(mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdee")), 0}: append([]byte(nil), enc...)}},
			Backend: BackendSoftware,
		}
		job.Execute()
		if job.Err != nil {
			t.Fatalf("execute: %v", job.Err)
		}
		got, ok := job.Result.(*DecodedFontLOD)
		if !ok {
			t.Fatalf("unexpected result type: %T", job.Result)
		}
		if !bytes.Equal(got.Data, raw) {
			t.Fatalf("unexpected payload: %q", got.Data)
		}
	})

	t.Run("svg lod2 color", func(t *testing.T) {
		var payload [4]byte
		binary.LittleEndian.PutUint32(payload[:], 0x11223344)
		job := &AssetLoadJob{
			ID:      mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdf0"),
			Type:    AssetTypeSVG,
			LOD:     2,
			Source:  staticSource{data: map[[2]uint64][]byte{{assetIDKey(mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdf0")), 2}: append([]byte(nil), payload[:]...)}},
			Backend: BackendSoftware,
		}
		job.Execute()
		if job.Err != nil {
			t.Fatalf("execute: %v", job.Err)
		}
		got, ok := job.Result.(*DecodedSVGLOD2)
		if !ok {
			t.Fatalf("unexpected result type: %T", job.Result)
		}
		if got.DominantColor != 0x11223344 {
			t.Fatalf("unexpected color: %#x", got.DominantColor)
		}
	})
}

func TestManagerScheduleAndDrainAsync(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdef")
	reg := NewAssetRegistryStore()
	configPayload := zstd.EncodeTo(nil, []byte("geometry-bytes"))
	src := &blockingSource{
		payload: configPayload,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	mgr := NewManagerImpl(reg, src, BackendSoftware, nil)

	start := time.Now()
	mgr.scheduleLOD(id, "assets/theme.toml", AssetTypeConfig, 0)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("schedule blocked too long: %v", elapsed)
	}

	select {
	case <-src.started:
	case <-time.After(time.Second):
		t.Fatal("job never started")
	}

	close(src.release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := mgr.DrainCompleted(); got > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	entry := reg.Get(id)
	if entry == nil {
		t.Fatal("expected registry entry")
	}
	if entry.State != AssetStateReady {
		t.Fatalf("unexpected state: %v", entry.State)
	}
	got, ok := entry.LODHandles[0].(*DecodedConfigLOD)
	if !ok {
		t.Fatalf("unexpected result type: %T", entry.LODHandles[0])
	}
	if !bytes.Equal(got.Data, []byte("geometry-bytes")) {
		t.Fatalf("unexpected committed bytes: %q", got.Data)
	}
}

func TestCommitJobRejectsStaleVersion(t *testing.T) {
	id := mustAssetID(t, "01234567-89ab-cdef-0123-456789abcdee")
	reg := NewAssetRegistryStore()
	mgr := NewManagerImpl(reg, staticSource{}, BackendSoftware, nil)

	reg.SetLODReady(id, 0, &DecodedSVGLOD0{Data: []byte("current")}, 10)
	entry := reg.Get(id)
	if entry == nil {
		t.Fatal("expected entry")
	}
	version := entry.EntryVersion

	job := &AssetLoadJob{
		ID:           id,
		Type:         AssetTypeSVG,
		LOD:          0,
		EntryVersion: version - 1,
		Result:       &DecodedSVGLOD0{Data: []byte("stale")},
		ElapsedNs:    20,
	}
	mgr.commitJob(job)

	entry = reg.Get(id)
	got, ok := entry.LODHandles[0].(*DecodedSVGLOD0)
	if !ok {
		t.Fatalf("unexpected type: %T", entry.LODHandles[0])
	}
	if string(got.Data) != "current" {
		t.Fatalf("stale commit overwrote payload: %q", got.Data)
	}
	if entry.EntryVersion != version {
		t.Fatalf("unexpected version after stale commit: %d", entry.EntryVersion)
	}
}

func TestWaitingOnSchedulesConfigAfterLastDependency(t *testing.T) {
	fontID := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc001")
	imageID := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc002")
	configID := mustAssetID(t, "01234567-89ab-cdef-0123-456789abc003")

	reg := NewAssetRegistryStore()
	sched := captureScheduler{jobs: make(chan *AssetLoadJob, 4)}
	mgr := NewManagerImpl(reg, staticSource{}, BackendSoftware, sched)
	mgr.SetDependencyTree(fakeConfigTree{nodes: map[AssetID]*ConfigNode{
		configID: {
			ID:   configID,
			Path: "assets/config/theme.toml",
			Deps: []AssetID{fontID, imageID},
		},
	}})

	mgr.scheduleConfig(configID, "assets/config/theme.toml")
	if got := len(sched.jobs); got != 0 {
		t.Fatalf("expected config to wait, got %d scheduled jobs", got)
	}

	reg.GetOrCreate(fontID)
	reg.GetOrCreate(imageID)

	mgr.commitJob(&AssetLoadJob{
		ID:           fontID,
		Type:         AssetTypeFont,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedFontLOD{Data: []byte("font")},
		ElapsedNs:    1,
	})
	if got := len(sched.jobs); got != 0 {
		t.Fatalf("expected config to still wait on image, got %d scheduled jobs", got)
	}

	mgr.commitJob(&AssetLoadJob{
		ID:           imageID,
		Type:         AssetTypeImage,
		LOD:          0,
		EntryVersion: 0,
		Result:       &DecodedImageLOD{Data: []byte("image")},
		ElapsedNs:    1,
	})

	select {
	case job := <-sched.jobs:
		if job.ID != configID {
			t.Fatalf("unexpected scheduled config: %s", job.ID)
		}
		if job.Type != AssetTypeConfig || job.LOD != 0 {
			t.Fatalf("unexpected config job: %+v", job)
		}
	case <-time.After(time.Second):
		t.Fatal("expected config to schedule after last dependency")
	}
}

type fakeConfigTree struct {
	nodes map[AssetID]*ConfigNode
}

func (f fakeConfigTree) ConfigNode(id AssetID) *ConfigNode {
	if f.nodes == nil {
		return nil
	}
	return f.nodes[id]
}

func mustAssetID(t *testing.T, s string) AssetID {
	t.Helper()
	id, err := ParseAssetID(s)
	if err != nil {
		t.Fatalf("parse asset id: %v", err)
	}
	return id
}
