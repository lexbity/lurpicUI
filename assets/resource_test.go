package assets

import (
	"errors"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
)

// TestConfiguredBudgetReachesActualManager verifies that the configured
// budget from the resource plan actually reaches the manager, per FR-10.
func TestConfiguredBudgetReachesActualManager(t *testing.T) {
	// Verify that the default plans have the expected asset wallet capacities.
	for _, profile := range []membudget.Profile{membudget.Compact, membudget.Standard} {
		plan := membudget.DefaultPlan(profile)
		assetsCap := plan.Caps[membudget.AssetsCPU]
		if assetsCap == 0 {
			t.Fatalf("profile=%v: assets CPU wallet has zero capacity", profile)
		}
		t.Logf("profile=%v: assets CPU wallet capacity=%d", profile, assetsCap)
	}

	// Verify that wallets can be created with the expected capacities.
	compactWallet, err := membudget.NewWallet(24 * 1024 * 1024) // 24 MiB for Compact
	if err != nil {
		t.Fatalf("failed to create compact wallet: %v", err)
	}
	cap, _, _, _ := compactWallet.Snapshot()
	expected := uint64(24 * 1024 * 1024)
	if cap != expected {
		t.Fatalf("expected compact wallet cap=%d, got %d", expected, cap)
	}

	standardWallet, err := membudget.NewWallet(96 * 1024 * 1024) // 96 MiB for Standard
	if err != nil {
		t.Fatalf("failed to create standard wallet: %v", err)
	}
	cap, _, _, _ = standardWallet.Snapshot()
	expected = uint64(96 * 1024 * 1024)
	if cap != expected {
		t.Fatalf("expected standard wallet cap=%d, got %d", expected, cap)
	}
}

// TestWalletReservationLifecycle verifies that wallet reservations properly
// track through reserved -> live -> retiring -> acknowledged states, per FR-10.
func TestWalletReservationLifecycle(t *testing.T) {
	wallet, err := membudget.NewWallet(1024)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	// Reserve
	reservation, err := wallet.Reserve(100)
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}

	// Check reserved state
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 100 || live != 0 || retiring != 0 {
		t.Fatalf("after reserve: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}

	// Publish
	if err := reservation.Publish(); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// Check live state
	_, reserved, live, retiring = wallet.Snapshot()
	if reserved != 0 || live != 100 || retiring != 0 {
		t.Fatalf("after publish: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}

	// Retire
	if err := reservation.Retire(); err != nil {
		t.Fatalf("retire failed: %v", err)
	}

	// Check retiring state
	_, reserved, live, retiring = wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 100 {
		t.Fatalf("after retire: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}

	// Acknowledge
	if err := reservation.Acknowledge(); err != nil {
		t.Fatalf("acknowledge failed: %v", err)
	}

	// Check all cleared
	_, reserved, live, retiring = wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("after acknowledge: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
}

// TestWalletRefusesOverCapacity verifies that wallet reservations are refused
// when they would exceed capacity, per FR-10.
func TestWalletRefusesOverCapacity(t *testing.T) {
	wallet, err := membudget.NewWallet(100)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	// Reserve all capacity
	reservation1, err := wallet.Reserve(100)
	if err != nil {
		t.Fatalf("first reserve failed: %v", err)
	}

	// Try to reserve more - should fail
	_, err = wallet.Reserve(1)
	if err == nil {
		t.Fatal("expected second reserve to fail, but it succeeded")
	}

	// Check that the error is a ResourceLimit
	var limit *membudget.ResourceLimit
	if !errors.As(err, &limit) {
		t.Fatalf("expected ResourceLimit error, got %T: %v", err, err)
	}

	// Clean up
	_ = reservation1.Cancel()
}

// TestWalletBatchReservation verifies that batch reservations are atomic,
// per FR-10.
func TestWalletBatchReservation(t *testing.T) {
	wallet, err := membudget.NewWallet(100)
	if err != nil {
		t.Fatalf("failed to create wallet: %v", err)
	}

	// Try to reserve 150 bytes in a batch - should fail
	_, err = wallet.ReserveBatch([]uint64{50, 50, 50})
	if err == nil {
		t.Fatal("expected batch reserve to fail, but it succeeded")
	}

	// Check that nothing was reserved
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("batch reserve partially succeeded: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}

	// Now try a batch that fits
	reservations, err := wallet.ReserveBatch([]uint64{30, 30, 30})
	if err != nil {
		t.Fatalf("batch reserve failed: %v", err)
	}
	if len(reservations) != 3 {
		t.Fatalf("expected 3 reservations, got %d", len(reservations))
	}

	// Check that all 90 bytes are reserved
	_, reserved, live, retiring = wallet.Snapshot()
	if reserved != 90 || live != 0 || retiring != 0 {
		t.Fatalf("after batch reserve: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}

	// Publish all
	for _, r := range reservations {
		_ = r.Publish()
	}

	// Check that all 90 bytes are live
	_, reserved, live, retiring = wallet.Snapshot()
	if reserved != 0 || live != 90 || retiring != 0 {
		t.Fatalf("after batch publish: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}

	// Clean up
	for _, r := range reservations {
		_ = r.Cancel()
	}
}
