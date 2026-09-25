package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TaskState string

const (
	StatePending           TaskState = "PENDING"
	StateDeliveryUncertain TaskState = "DELIVERY_UNCERTAIN"
	StateWaitingRetry      TaskState = "WAITING_RETRY"
	StateDelivered         TaskState = "DELIVERED"
	StateCompleted         TaskState = "COMPLETED"
	StateCancelled         TaskState = "CANCELLED"
	StateCancelUnconfirmed TaskState = "CANCELLED_LOCAL_AO_STOP_UNCONFIRMED"
	StateBlocked           TaskState = "BLOCKED"
)

type SideEffectState string

const (
	SideEffectPending   SideEffectState = "PENDING"
	SideEffectCompleted SideEffectState = "COMPLETED"
)

// Checkpoint is the durable, non-secret recovery contract for one bounded task.
// AO remains authoritative for session and delivery state; this record only
// controls whether the next action is eligible to be sent.
type Checkpoint struct {
	Version          int             `json:"version"`
	TaskID           string          `json:"taskId"`
	DeliveryID       string          `json:"deliveryId"`
	SessionID        string          `json:"sessionId"`
	ProjectID        string          `json:"projectId,omitempty"`
	SessionKind      string          `json:"sessionKind,omitempty"`
	SessionHarness   string          `json:"sessionHarness,omitempty"`
	SessionModel     string          `json:"sessionModel,omitempty"`
	SessionEffort    string          `json:"sessionEffort,omitempty"`
	SessionBranch    string          `json:"sessionBranch,omitempty"`
	RunOwner         string          `json:"runOwner,omitempty"`
	SessionExclusive bool            `json:"sessionExclusive,omitempty"`
	AOTurnID         string          `json:"aoTurnId,omitempty"`
	ArtifactPath     string          `json:"artifactPath"`
	ArtifactSHA256   string          `json:"artifactSha256"`
	GitHead          string          `json:"gitHead"`
	NextAction       string          `json:"nextAction"`
	State            TaskState       `json:"state"`
	SideEffect       SideEffectState `json:"sideEffect"`
	RetryBudget      int             `json:"retryBudget"`
	Attempts         int             `json:"attempts"`
	NextRetryAt      *time.Time      `json:"nextRetryAt,omitempty"`
	LastErrorKind    string          `json:"lastErrorKind,omitempty"`
	LastObservedUTC  time.Time       `json:"lastObservedUtc"`
}

func (c Checkpoint) Terminal() bool {
	return c.State == StateBlocked || c.State == StateCancelled || c.State == StateCancelUnconfirmed || c.State == StateCompleted || c.SideEffect == SideEffectCompleted
}

func (c Checkpoint) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported checkpoint version %d", c.Version)
	}
	if c.TaskID == "" || c.DeliveryID == "" || c.SessionID == "" {
		return errors.New("taskId, deliveryId and sessionId are required")
	}
	if c.ArtifactPath == "" || c.ArtifactSHA256 == "" || c.GitHead == "" {
		return errors.New("artifactPath, artifactSha256 and gitHead are required")
	}
	if c.RetryBudget < 0 || c.Attempts < 0 {
		return errors.New("retry counters cannot be negative")
	}
	return nil
}

type FileStore struct{ Path string }

type GitArtifactReader struct{ Root string }

func (r GitArtifactReader) Observe(ctx context.Context, name string) (ArtifactObservation, error) {
	root, err := filepath.Abs(r.Root)
	if err != nil {
		return ArtifactObservation{}, err
	}
	target, err := filepath.Abs(filepath.Join(root, name))
	if err != nil {
		return ArtifactObservation{}, err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return ArtifactObservation{}, errors.New("artifact path escapes root")
	}
	f, err := os.Open(target)
	if err != nil {
		return ArtifactObservation{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ArtifactObservation{}, err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	head, err := cmd.Output()
	if err != nil {
		return ArtifactObservation{}, fmt.Errorf("read git head: %w", err)
	}
	branchCmd := exec.CommandContext(ctx, "git", "-C", root, "branch", "--show-current")
	branch, err := branchCmd.Output()
	if err != nil {
		return ArtifactObservation{}, fmt.Errorf("read git branch: %w", err)
	}
	return ArtifactObservation{SHA256: fmt.Sprintf("%x", h.Sum(nil)), GitHead: strings.TrimSpace(string(head)), Branch: strings.TrimSpace(string(branch))}, nil
}

func (s FileStore) Load() (Checkpoint, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return Checkpoint{}, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return Checkpoint{}, fmt.Errorf("decode checkpoint: %w", err)
	}
	return cp, cp.Validate()
}

func (s FileStore) Save(cp Checkpoint) error {
	if err := cp.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".checkpoint-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.Path)
}
