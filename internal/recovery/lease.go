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

// FileLease serializes every dispatcher owner that uses the run-owned wrapper.
// Its O_EXCL create spans AO preflight and POST, closing the wrapper-to-wrapper
// race without changing AO itself. It is deliberately not a global AO lock.
type FileLease struct {
	root    string
	ownerID string
}

func NewFileLease(workspaceRoot, ownerID string) (FileLease, error) {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return FileLease{}, err
	}
	if ownerID == "" {
		return FileLease{}, errors.New("owner id is required")
	}
	return FileLease{root: filepath.Join(root, ".agents-coworkers", "recovery-leases"), ownerID: ownerID}, nil
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
