package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrLeaseHeld = errors.New("recovery lease is held by another owner")

// LeaseState represents the observed state of a run lease in Slice 8A.
type LeaseState string

const (
	LeaseStateFree   LeaseState = "FREE"
	LeaseStateOwned  LeaseState = "OWNED"
	LeaseStateLocked LeaseState = "LOCKED"
)

// LeaseInspection captures the read-only inspection facts of a lease.
type LeaseInspection struct {
	WorkspaceRoot string     `json:"workspaceRoot"`
	RunOwner      string     `json:"runOwner"`
	TaskID        string     `json:"taskId"`
	OwnerID       string     `json:"ownerId"`
	ObservedState LeaseState `json:"observedState"`
	ObservedPID   int        `json:"observedPid"`
	FilePath      string     `json:"filePath,omitempty"`
}

// FileLease serializes every dispatcher owner that uses the run-owned wrapper.
// Its O_EXCL create spans AO preflight and POST, closing the wrapper-to-wrapper
// race without changing AO itself. It is deliberately not a global AO lock.
type FileLease struct {
	workspaceRoot string
	root          string
	ownerID       string
}

func NewFileLease(workspaceRoot, ownerID string) (FileLease, error) {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return FileLease{}, err
	}
	if ownerID == "" {
		return FileLease{}, errors.New("owner id is required")
	}
	return FileLease{
		workspaceRoot: root,
		root:          filepath.Join(root, ".agents-coworkers", "recovery-leases"),
		ownerID:       ownerID,
	}, nil
}

type leaseRecord struct {
	OwnerID string `json:"ownerId"`
	TaskID  string `json:"taskId"`
	PID     int    `json:"pid"`
}

func (l FileLease) Acquire(ctx context.Context, runOwner, taskID string) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l.root == "" || runOwner == "" || l.ownerID == "" || taskID == "" {
		return nil, errors.New("lease root, run owner, owner id and task id are required")
	}
	root := l.root
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	key := sha256.Sum256([]byte(runOwner + "\n" + taskID))
	path := filepath.Join(root, fmt.Sprintf("%x.lease", key))
	record := leaseRecord{OwnerID: l.ownerID, TaskID: taskID, PID: os.Getpid()}
	data, _ := json.Marshal(record)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, ErrLeaseHeld
	}
	if err != nil {
		return nil, err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return func() error {
		current, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var held leaseRecord
		if json.Unmarshal(current, &held) != nil || held.OwnerID != l.ownerID || held.TaskID != taskID {
			return ErrLeaseHeld
		}
		return os.Remove(path)
	}, nil
}

// Inspect performs a read-only inspection of the lease for the given runOwner and taskID.
func (l FileLease) Inspect(runOwner, taskID string) (LeaseInspection, error) {
	return InspectLease(l.workspaceRoot, runOwner, taskID, l.ownerID)
}

// InspectLease performs a read-only inspection of the recovery lease given workspaceRoot,
// runOwner, taskID, and the expected ownerID. It does not acquire, mutate, or release the lease.
func InspectLease(workspaceRoot, runOwner, taskID, expectedOwnerID string) (LeaseInspection, error) {
	if workspaceRoot == "" || runOwner == "" || taskID == "" {
		return LeaseInspection{}, errors.New("workspace root, run owner, and task id are required for lease inspection")
	}
	canonicalRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return LeaseInspection{}, fmt.Errorf("canonicalize workspace root: %w", err)
	}
	root := filepath.Join(canonicalRoot, ".agents-coworkers", "recovery-leases")
	key := sha256.Sum256([]byte(runOwner + "\n" + taskID))
	path := filepath.Join(root, fmt.Sprintf("%x.lease", key))

	res := LeaseInspection{
		WorkspaceRoot: canonicalRoot,
		RunOwner:      runOwner,
		TaskID:        taskID,
		OwnerID:       expectedOwnerID,
		ObservedState: LeaseStateFree,
		ObservedPID:   0,
		FilePath:      path,
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return res, nil
	}
	if err != nil {
		return LeaseInspection{}, fmt.Errorf("read lease file: %w", err)
	}

	var held leaseRecord
	if err := json.Unmarshal(data, &held); err != nil {
		return LeaseInspection{}, fmt.Errorf("malformed lease record in %s: %w", path, err)
	}
	if held.OwnerID == "" || held.TaskID == "" {
		return LeaseInspection{}, fmt.Errorf("malformed lease record in %s: missing owner or task id", path)
	}

	res.ObservedPID = held.PID
	res.OwnerID = held.OwnerID
	if held.OwnerID == expectedOwnerID && held.TaskID == taskID {
		res.ObservedState = LeaseStateOwned
	} else {
		res.ObservedState = LeaseStateLocked
	}
	return res, nil
}
