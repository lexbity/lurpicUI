package runtime

import (
	"errors"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/render"
)

// frameStorage fixes packet capacity at startup. Ordinary/latest-ready pages
// cannot consume the cleanup partition, even when a capture holds old pages.
// All partitions report through wallet and share its allocation identities.
type frameStorage struct {
	wallet    *membudget.Wallet
	ordinary  *membudget.Wallet
	cleanup   *membudget.Wallet
	scratch   *membudget.Wallet
	packetCap uint64
}

type frameSealScratch struct {
	keys   [render.MaxFrameWindows]string
	sizes  [render.MaxFrameWindows]uint64
	frames [render.MaxFrameWindows]*render.Frame
}

const frameSealScratchBytes = uint64(unsafe.Sizeof(frameSealScratch{})) + render.MaxFrameWindows*uint64(unsafe.Sizeof((*membudget.Reservation)(nil)))

func framePacketCapacity(profile membudget.Profile) uint64 {
	if profile == membudget.Compact {
		return 2 << 20
	}
	return 8 << 20
}

func newFrameStorage(capacity, packetCap uint64) (*frameStorage, error) {
	// Two ordinary in-flight groups, latest-ready, cleanup, and independent
	// scratch must all fit without borrowing. Divide before multiplying.
	if capacity <= frameSealScratchBytes || packetCap == 0 || packetCap > (capacity-frameSealScratchBytes)/4 {
		return nil, errors.New("runtime: frame capacity must fit four packet groups and scratch")
	}
	wallet, err := membudget.NewWallet(capacity)
	if err != nil {
		return nil, err
	}
	partitions, err := wallet.Partition(3*packetCap, packetCap, capacity-4*packetCap)
	if err != nil {
		return nil, err
	}
	return &frameStorage{wallet: wallet, ordinary: partitions[0], cleanup: partitions[1], scratch: partitions[2], packetCap: packetCap}, nil
}

func (s *frameStorage) reserve(sizes []uint64, cleanup bool) ([]*membudget.Reservation, error) {
	if cleanup {
		return s.cleanup.ReserveBatch(sizes)
	}
	return s.ordinary.ReserveBatch(sizes)
}
