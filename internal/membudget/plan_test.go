package membudget

import (
	"errors"
	"math"
	"testing"
)

func TestWalletPartitionsProtectCapacityAndIdentity(t *testing.T) {
	w, err := NewWallet(100)
	if err != nil {
		t.Fatal(err)
	}
	children, err := w.Partition(70, 30)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := children[0].Reserve(70)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := children[0].Reserve(1); err == nil {
		t.Fatal("ordinary owner borrowed cleanup capacity")
	}
	if _, err := w.Reserve(1); err == nil {
		t.Fatal("aggregate wallet bypassed its fixed partitions")
	}
	cleanup, err := children[1].Reserve(30)
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.ID() == cleanup.ID() {
		t.Fatal("sibling reservations reused a generation")
	}
	if err := ordinary.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := ordinary.Retire(); err != nil {
		t.Fatal(err)
	}
	_, reserved, live, retiring := w.Snapshot()
	if reserved != 30 || live != 0 || retiring != 70 {
		t.Fatalf("aggregate charges = %d/%d/%d", reserved, live, retiring)
	}
	// A forged sibling token must not release an otherwise valid generation.
	foreign := &Reservation{wallet: children[1], id: ordinary.ID()}
	if foreign.Bytes() != 0 || foreign.Acknowledge() == nil {
		t.Fatal("sibling released another owner's allocation")
	}
	if err := ordinary.Acknowledge(); err != nil {
		t.Fatal(err)
	}
	replacement, err := children[0].Reserve(70)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.ID() <= cleanup.ID() || ordinary.Acknowledge() == nil {
		t.Fatal("late acknowledgment affected a replacement")
	}
	if err := cleanup.Cancel(); err != nil {
		t.Fatal(err)
	}
	if err := replacement.Cancel(); err != nil {
		t.Fatal(err)
	}
	_, reserved, live, retiring = w.Snapshot()
	if reserved+live+retiring != 0 {
		t.Fatal("partition release did not balance aggregate charges")
	}
}

func TestWalletPartitionRequiresUnallocatedCompletePlan(t *testing.T) {
	for _, capacities := range [][]uint64{{}, {0, 100}, {50, 49}, {50, 51}, {math.MaxUint64, 1}} {
		w, err := NewWallet(100)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Partition(capacities...); err == nil {
			t.Fatalf("accepted partition %v", capacities)
		}
		reservation, err := w.Reserve(1)
		if err != nil {
			t.Fatal("failed partition changed the original wallet", err)
		}
		if err := reservation.Cancel(); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Partition(100); err == nil {
			t.Fatal("partition accepted after allocation history")
		}
	}
}

func TestBatchReservationRefusalIsAtomic(t *testing.T) {
	wallet, err := NewWallet(10)
	if err != nil {
		t.Fatal(err)
	}
	_, err = wallet.ReserveBatch([]uint64{7, 7})
	var limit *ResourceLimit
	if !errors.As(err, &limit) {
		t.Fatalf("batch refusal=%v, want typed resource limit", err)
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved+live+retiring != 0 {
		t.Fatal("batch partially admitted")
	}
	reservations, err := wallet.ReserveBatch([]uint64{4, 6})
	if err != nil {
		t.Fatal(err)
	}
	if reservations[0].ID() != 1 || reservations[1].ID() != 2 {
		t.Fatal("refusal consumed allocation identities")
	}
	for _, reservation := range reservations {
		if err := reservation.Cancel(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProfilesPartitionFiniteCapacity(t *testing.T) {
	compact, standard := DefaultPlan(Compact), DefaultPlan(Standard)
	if err := compact.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := standard.Validate(); err != nil {
		t.Fatal(err)
	}
	for i := range compact.Caps {
		if standard.Caps[i] < compact.Caps[i] {
			t.Fatalf("standard wallet %d is smaller", i)
		}
	}
	bad := standard
	bad.Caps[Text] = math.MaxUint64
	bad.Caps[GPU] = 1
	if err := bad.Validate(); err == nil {
		t.Fatal("aggregate overflow was accepted")
	}
}

func TestWalletOwnershipTransitionsConserveCharge(t *testing.T) {
	w, err := NewWallet(100)
	if err != nil {
		t.Fatal(err)
	}
	retiringReservation, err := w.Reserve(30)
	if err != nil {
		t.Fatal(err)
	}
	if err := retiringReservation.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := retiringReservation.Retire(); err != nil {
		t.Fatal(err)
	}
	if err := retiringReservation.Release(); err == nil {
		t.Fatal("retiring allocation was released before acknowledgment")
	}
	if err := retiringReservation.Acknowledge(); err != nil {
		t.Fatal(err)
	}
	if err := retiringReservation.Acknowledge(); err == nil {
		t.Fatal("duplicate acknowledgment was accepted")
	}
	liveReservation, err := w.Reserve(50)
	if err != nil {
		t.Fatal(err)
	}
	if err := liveReservation.Publish(); err != nil {
		t.Fatal(err)
	}
	if err := retiringReservation.Release(); err == nil {
		t.Fatal("stale allocation token released a replacement")
	}
	if err := liveReservation.Release(); err != nil {
		t.Fatal(err)
	}
	cap, reserved, live, retiring := w.Snapshot()
	if cap != 100 || reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("wallet = %d/%d/%d/%d", cap, reserved, live, retiring)
	}
}

func TestWalletRefusesOverCapacityBeforeClaim(t *testing.T) {
	w, err := NewWallet(10)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := w.Reserve(10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Reserve(1); err == nil {
		t.Fatal("over-capacity reservation accepted")
	}
	_, reserved, live, retiring := w.Snapshot()
	if reserved != 10 || live != 0 || retiring != 0 {
		t.Fatalf("failed reservation changed charge: %d/%d/%d", reserved, live, retiring)
	}
	if err := reservation.Cancel(); err != nil {
		t.Fatal(err)
	}
}

func TestWalletCancelsUnpublishedScratch(t *testing.T) {
	wallet, err := NewWallet(64)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := wallet.Reserve(32)
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Cancel(); err != nil {
		t.Fatal(err)
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("after scratch cancellation: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
	if err := reservation.Cancel(); err == nil {
		t.Fatal("duplicate reservation cancellation was accepted")
	}
}
