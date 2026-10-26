package membudget

import (
	"math"
	"testing"
)

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
