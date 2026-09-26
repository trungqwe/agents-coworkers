package recovery

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFileLease_LifecycleAndInspection(t *testing.T) {
	tmpDir := t.TempDir()
	ownerID := "run-test-owner-001"
	runOwner := "run-test-owner-001"
	taskID := "__run__"

	lease, err := NewFileLease(tmpDir, ownerID)
	if err != nil {
		t.Fatalf("NewFileLease failed: %v", err)
	}

	// 1. Initially FREE
	insp, err := lease.Inspect(runOwner, taskID)
	if err != nil {
		t.Fatalf("Inspect failed initially: %v", err)
	}
	if insp.ObservedState != LeaseStateFree {
		t.Fatalf("expected state FREE, got %s", insp.ObservedState)
	}
	if insp.ObservedPID != 0 {
		t.Fatalf("expected PID 0 when FREE, got %d", insp.ObservedPID)
	}

	// 2. Standalone InspectLease also returns FREE
	inspDirect, err := InspectLease(tmpDir, runOwner, taskID, ownerID)
	if err != nil {
		t.Fatalf("InspectLease failed: %v", err)
	}
	if inspDirect.ObservedState != LeaseStateFree {
		t.Fatalf("expected state FREE from direct inspect, got %s", inspDirect.ObservedState)
	}

	// 3. Acquire lease
	ctx := context.Background()
	release, err := lease.Acquire(ctx, runOwner, taskID)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer func() {
		if release != nil {
			_ = release()
		}
	}()

	// 4. While held by owner -> Inspect returns OWNED
	insp, err = lease.Inspect(runOwner, taskID)
	if err != nil {
		t.Fatalf("Inspect while held failed: %v", err)
	}
	if insp.ObservedState != LeaseStateOwned {
		t.Fatalf("expected state OWNED, got %s", insp.ObservedState)
	}
	if insp.ObservedPID != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), insp.ObservedPID)
	}

	// 5. Another owner inspects -> returns LOCKED
	otherOwnerID := "different-run-owner"
	otherLease, err := NewFileLease(tmpDir, otherOwnerID)
	if err != nil {
		t.Fatalf("NewFileLease for other owner failed: %v", err)
	}
	inspOther, err := otherLease.Inspect(runOwner, taskID)
	if err != nil {
		t.Fatalf("Inspect by other owner failed: %v", err)
	}
	if inspOther.ObservedState != LeaseStateLocked {
		t.Fatalf("expected state LOCKED for other owner, got %s", inspOther.ObservedState)
	}
	if inspOther.OwnerID != ownerID {
		t.Fatalf("expected OwnerID %s, got %s", ownerID, inspOther.OwnerID)
	}

	// 6. Another owner tries to Acquire -> ErrLeaseHeld
	_, err = otherLease.Acquire(ctx, runOwner, taskID)
	if err != ErrLeaseHeld {
		t.Fatalf("expected ErrLeaseHeld, got %v", err)
	}

	// 7. Release lease
	if err := release(); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	release = nil

	// 8. After release -> FREE again
	insp, err = lease.Inspect(runOwner, taskID)
	if err != nil {
		t.Fatalf("Inspect after release failed: %v", err)
	}
	if insp.ObservedState != LeaseStateFree {
		t.Fatalf("expected state FREE after release, got %s", insp.ObservedState)
	}
}

func TestInspectLease_MalformedFailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	runOwner := "run-001"
	taskID := "__run__"

	leaseRoot := filepath.Join(tmpDir, ".agents-coworkers", "recovery-leases")
	if err := os.MkdirAll(leaseRoot, 0o700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	key := sha256.Sum256([]byte(runOwner + "\n" + taskID))
	path := filepath.Join(leaseRoot, fmt.Sprintf("%x.lease", key))

	// Write garbage data
	if err := os.WriteFile(path, []byte("not-valid-json"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := InspectLease(tmpDir, runOwner, taskID, runOwner)
	if err == nil {
		t.Fatal("expected error for malformed lease file, got nil")
	}

	// Write json missing taskID
	if err := os.WriteFile(path, []byte(`{"ownerId":"run-001"}`), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	_, err = InspectLease(tmpDir, runOwner, taskID, runOwner)
	if err == nil {
		t.Fatal("expected error for lease missing taskId, got nil")
	}
}
