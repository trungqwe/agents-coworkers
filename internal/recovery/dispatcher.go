package recovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type DeliveryState string

const (
	DeliveryNotFound  DeliveryState = "NOT_FOUND"
	DeliveryReserved  DeliveryState = "RESERVED"
	DeliveryAccepted  DeliveryState = "ACCEPTED"
	DeliveryCompleted DeliveryState = "COMPLETED"
	DeliveryRejected  DeliveryState = "REJECTED"
	DeliveryStopped   DeliveryState = "STOPPED"
)

type Observation struct {
	SessionExists bool
	HumanStopped  bool
	Delivery      DeliveryState
	Receipt       *TaskReceipt
}

type TaskReceipt struct {
	TaskID         string `json:"taskId"`
	ArtifactSHA256 string `json:"artifactSha256"`
	Accepted       bool   `json:"accepted"`
}

type AO interface {
	Observe(context.Context, string, string) (Observation, error)
	Preflight(context.Context, SessionExpectation) error
	Send(context.Context, string, string, string) (DeliveryReceipt, error)
	RecoverOnly(context.Context, string, string) (DeliveryReceipt, error)
	Stop(context.Context, string, string) error
}

type SessionExpectation struct {
	SessionID string
	ProjectID string
	Kind      string
	Harness   string
	Model     string
	Effort    string
	Branch    string
	Exclusive bool
}

type Lease interface {
	Acquire(context.Context, string, string) (func() error, error)
}

type DeliveryReceipt struct {
	State  DeliveryState
	TurnID string
}

type ArtifactReader interface {
	Observe(context.Context, string) (ArtifactObservation, error)
}

type ArtifactObservation struct {
	SHA256  string
	GitHead string
	Branch  string
}

type Store interface {
	Load() (Checkpoint, error)
	Save(Checkpoint) error
}

var (
	ErrProviderUnavailable  = errors.New("provider unavailable")
	ErrDeliveryUncertain    = errors.New("delivery uncertain")
	ErrRequestNotAccepted   = errors.New("request definitely not accepted")
	ErrSessionContaminated  = errors.New("AO steered delivery into an existing turn")
	ErrSessionNotIdle       = errors.New("AO session is not idle")
	ErrSessionOwnerMismatch = errors.New("AO session ownership mismatch")
)

type Dispatcher struct {
	Store      Store
	AO         AO
	Artifacts  ArtifactReader
	Now        func() time.Time
	RetryDelay time.Duration
	Lease      Lease
	mu         sync.Mutex
}

func (d *Dispatcher) Step(ctx context.Context) (Checkpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stepLocked(ctx)
}

func (d *Dispatcher) Cancel(ctx context.Context, now time.Time) (Checkpoint, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	release, err := d.acquire(ctx)
	if err != nil {
		return Checkpoint{}, err
	}
	defer release()
	cp, err := d.Store.Load()
	if err != nil {
		return Checkpoint{}, err
	}
	if cp.State == StateCompleted || cp.State == StateBlocked || cp.State == StateCancelled {
		return cp, nil
	}
	if cp.AOTurnID == "" && (cp.State == StatePending || cp.State == StateWaitingRetry) {
		cp.State = StateCancelled
		cp.NextRetryAt = nil
		cp.LastErrorKind = "HUMAN_STOP"
		cp.LastObservedUTC = now.UTC()
		return cp, d.Store.Save(cp)
	}
	// Persist local cancellation before any recovery or interrupt I/O. Step treats
	// this state as terminal; only an explicit later Cancel call may reconcile it.
	cp.State = StateCancelUnconfirmed
	cp.NextRetryAt = nil
	cp.LastErrorKind = "AO_STOP_UNCONFIRMED"
	cp.LastObservedUTC = now.UTC()
	if err := d.Store.Save(cp); err != nil {
		return cp, err
	}
	if cp.AOTurnID == "" {
		receipt, recoverErr := d.AO.RecoverOnly(ctx, cp.SessionID, cp.DeliveryID)
		if recoverErr != nil {
			return cp, recoverErr
		}
		if receipt.TurnID == "" || (receipt.State != DeliveryAccepted && receipt.State != DeliveryCompleted && receipt.State != DeliveryStopped && receipt.State != DeliveryRejected) {
			return cp, ErrDeliveryUncertain
		}
		cp.AOTurnID = receipt.TurnID
		if err := d.Store.Save(cp); err != nil {
			return cp, err
		}
	}
	obs, observeErr := d.AO.Observe(ctx, cp.SessionID, cp.AOTurnID)
	if observeErr != nil {
		return cp, observeErr
	}
	if settled, settledErr, done := d.settleCancelObservation(cp, obs); done {
		return settled, settledErr
	}
	if !cp.SessionExclusive || cp.RunOwner == "" {
		return cp, fmt.Errorf("%w: session exclusivity is not proven", ErrSessionOwnerMismatch)
	}
	if err := d.AO.Stop(ctx, cp.SessionID, cp.AOTurnID); err != nil {
		obs, observeErr = d.AO.Observe(ctx, cp.SessionID, cp.AOTurnID)
		if observeErr == nil {
			if settled, settledErr, done := d.settleCancelObservation(cp, obs); done {
				return settled, settledErr
			}
		}
		return cp, err
	}
	obs, observeErr = d.AO.Observe(ctx, cp.SessionID, cp.AOTurnID)
	if observeErr != nil {
		return cp, observeErr
	}
	if settled, settledErr, done := d.settleCancelObservation(cp, obs); done {
		return settled, settledErr
	}
	return cp, ErrDeliveryUncertain
}

func (d *Dispatcher) settleCancelObservation(cp Checkpoint, obs Observation) (Checkpoint, error, bool) {
	if !obs.SessionExists {
		return cp, errors.New("AO session not found while reconciling Stop"), false
	}
	switch obs.Delivery {
	case DeliveryStopped:
		cp.State = StateCancelled
		cp.LastErrorKind = "HUMAN_STOP"
		return cp, d.Store.Save(cp), true
	case DeliveryCompleted:
		if obs.Receipt != nil && obs.Receipt.Accepted && obs.Receipt.TaskID == cp.TaskID && obs.Receipt.ArtifactSHA256 == cp.ArtifactSHA256 {
			settled, err := d.acceptReceipt(cp, obs.Receipt)
			return settled, err, true
		}
		cp.State = StateCancelUnconfirmed
		cp.SideEffect = SideEffectPending
		cp.LastErrorKind = "COMPLETED_OUTCOME_UNVERIFIED"
		return cp, d.Store.Save(cp), true
	case DeliveryRejected:
		cp.State = StateBlocked
		cp.LastErrorKind = "DELIVERY_FAILED"
		return cp, d.Store.Save(cp), true
	default:
		return cp, nil, false
	}
}

func (d *Dispatcher) stepLocked(ctx context.Context) (Checkpoint, error) {
	release, err := d.acquire(ctx)
	if err != nil {
		return Checkpoint{}, err
	}
	defer release()
	cp, err := d.Store.Load()
	if err != nil {
		return Checkpoint{}, err
	}
	now := d.now()
	cp.LastObservedUTC = now.UTC()

	// User cancellation is terminal and is checked before any external call.
	if cp.Terminal() {
		return cp, d.Store.Save(cp)
	}
	if (cp.State == StateDeliveryUncertain || cp.State == StateDelivered) && cp.NextRetryAt != nil && now.Before(*cp.NextRetryAt) {
		return cp, nil
	}

	artifact, err := d.Artifacts.Observe(ctx, cp.ArtifactPath)
	if err != nil {
		return d.block(cp, "ARTIFACT_READ_FAILED", err)
	}
	if artifact.SHA256 != cp.ArtifactSHA256 {
		return d.block(cp, "ARTIFACT_CHANGED", fmt.Errorf("artifact hash mismatch"))
	}
	if artifact.GitHead != cp.GitHead {
		return d.block(cp, "GIT_HEAD_CHANGED", fmt.Errorf("git head mismatch"))
	}
	if cp.SessionBranch != "" && artifact.Branch != cp.SessionBranch {
		return d.block(cp, "WORKTREE_BRANCH_MISMATCH", fmt.Errorf("artifact branch does not match AO session branch"))
	}

	obs, err := d.AO.Observe(ctx, cp.SessionID, cp.AOTurnID)
	if err != nil {
		cp.Attempts++
		if cp.State == StateDelivered || cp.State == StateDeliveryUncertain {
			return d.scheduleReconciliation(cp, now, "AO_OBSERVE_FAILED", err)
		}
		return d.scheduleSendRetry(cp, now, "AO_OBSERVE_FAILED", err)
	}
	if !obs.SessionExists {
		return d.block(cp, "SESSION_NOT_FOUND", errors.New("AO session not found"))
	}
	if obs.HumanStopped {
		cp.State = StateCancelled
		cp.NextRetryAt = nil
		cp.LastErrorKind = "HUMAN_STOP"
		return cp, d.Store.Save(cp)
	}
	if cp.State == StateDeliveryUncertain {
		receipt, recoverErr := d.AO.RecoverOnly(ctx, cp.SessionID, cp.DeliveryID)
		if recoverErr != nil {
			if errors.Is(recoverErr, ErrSessionContaminated) {
				return d.block(cp, "SESSION_CONTAMINATED", recoverErr)
			}
			return d.reconcileLater(cp, now, "DELIVERY_UNCERTAIN", recoverErr, true)
		}
		if receipt.State == DeliveryAccepted || receipt.State == DeliveryCompleted {
			cp.State = StateDelivered
			cp.AOTurnID = receipt.TurnID
			cp.NextRetryAt = nil
			cp.LastErrorKind = ""
			return cp, d.Store.Save(cp)
		}
		return d.reconcileLater(cp, now, "DELIVERY_NOT_CONFIRMED", ErrDeliveryUncertain, true)
	}
	if cp.State == StateDelivered && obs.Delivery == DeliveryNotFound {
		return d.block(cp, "DELIVERY_RECEIPT_LOST", errors.New("previously delivered task has no AO receipt"))
	}

	switch obs.Delivery {
	case DeliveryCompleted:
		return d.acceptReceipt(cp, obs.Receipt)
	case DeliveryAccepted:
		cp.State = StateDelivered
		cp.NextRetryAt = nil
		cp.LastErrorKind = ""
		return cp, d.Store.Save(cp)
	case DeliveryReserved:
		receipt, recoverErr := d.AO.RecoverOnly(ctx, cp.SessionID, cp.DeliveryID)
		if recoverErr != nil {
			if errors.Is(recoverErr, ErrSessionContaminated) {
				return d.block(cp, "SESSION_CONTAMINATED", recoverErr)
			}
			return d.reconcileLater(cp, now, "DELIVERY_UNCERTAIN", recoverErr, true)
		}
		if receipt.State == DeliveryAccepted || receipt.State == DeliveryCompleted {
			cp.State = StateDelivered
			cp.AOTurnID = receipt.TurnID
			cp.NextRetryAt = nil
			cp.LastErrorKind = ""
			return cp, d.Store.Save(cp)
		}
		return d.reconcileLater(cp, now, "DELIVERY_UNCERTAIN", ErrDeliveryUncertain, true)
	case DeliveryRejected:
		return d.block(cp, "DELIVERY_REJECTED", errors.New("AO rejected delivery"))
	case DeliveryStopped:
		cp.State = StateCancelled
		cp.NextRetryAt = nil
		cp.LastErrorKind = "HUMAN_STOP"
		return cp, d.Store.Save(cp)
	case DeliveryNotFound:
		// Only a missing durable AO receipt permits the first send. A restart will
		// observe the same delivery ID before deciding again.
		if cp.State == StateWaitingRetry && cp.NextRetryAt != nil && now.Before(*cp.NextRetryAt) {
			return cp, nil
		}
		if cp.Attempts >= cp.RetryBudget {
			return d.block(cp, "RETRY_BUDGET_EXHAUSTED", errors.New("retry budget exhausted"))
		}
		if err := d.AO.Preflight(ctx, SessionExpectation{SessionID: cp.SessionID, ProjectID: cp.ProjectID, Kind: cp.SessionKind, Harness: cp.SessionHarness, Model: cp.SessionModel, Effort: cp.SessionEffort, Branch: cp.SessionBranch, Exclusive: cp.SessionExclusive}); err != nil {
			return d.block(cp, "SESSION_PREFLIGHT_FAILED", err)
		}
		cp.Attempts++
		// Write ahead before provider I/O. If this process dies after AO accepts
		// the request, the next owner can only use recover-only with this ID.
		next := now.Add(d.retryDelay()).UTC()
		cp.State = StateDeliveryUncertain
		cp.NextRetryAt = &next
		cp.LastErrorKind = "SEND_IN_FLIGHT"
		if err := d.Store.Save(cp); err != nil {
			return cp, err
		}
		receipt, err := d.AO.Send(ctx, cp.SessionID, cp.DeliveryID, cp.NextAction)
		if err != nil {
			if errors.Is(err, ErrSessionContaminated) {
				return d.block(cp, "SESSION_CONTAMINATED", err)
			}
			if errors.Is(err, ErrRequestNotAccepted) {
				return d.scheduleSendRetry(cp, now, "REQUEST_NOT_ACCEPTED", err)
			}
			// Any other transport/provider error is uncertain: the request may have
			// crossed AO's durable boundary. Only recover-only may reconcile it.
			return d.reconcileLater(cp, now, "DELIVERY_UNCERTAIN", err, false)
		}
		cp.State = StateDelivered
		cp.AOTurnID = receipt.TurnID
		cp.NextRetryAt = nil
		cp.LastErrorKind = ""
		return cp, d.Store.Save(cp)
	default:
		return d.block(cp, "UNKNOWN_DELIVERY_STATE", fmt.Errorf("unknown delivery state %q", obs.Delivery))
	}
}

func (d *Dispatcher) scheduleReconciliation(cp Checkpoint, now time.Time, kind string, cause error) (Checkpoint, error) {
	if cp.Attempts >= cp.RetryBudget {
		return d.block(cp, "RETRY_BUDGET_EXHAUSTED", cause)
	}
	next := now.Add(d.retryDelay()).UTC()
	// Crucially preserve DELIVERED versus DELIVERY_UNCERTAIN. The former may
	// only be observed; the latter may only use recover-only.
	cp.NextRetryAt = &next
	cp.LastErrorKind = kind
	if err := d.Store.Save(cp); err != nil {
		return cp, err
	}
	return cp, cause
}

func (d *Dispatcher) acquire(ctx context.Context) (func() error, error) {
	if d.Lease == nil {
		return func() error { return nil }, nil
	}
	cp, err := d.Store.Load()
	if err != nil {
		return nil, err
	}
	return d.Lease.Acquire(ctx, cp.RunOwner, cp.TaskID)
}

func (d *Dispatcher) acceptReceipt(cp Checkpoint, receipt *TaskReceipt) (Checkpoint, error) {
	if receipt == nil {
		return d.block(cp, "TASK_RECEIPT_MISSING", errors.New("AO turn completed without task receipt"))
	}
	if !receipt.Accepted || receipt.TaskID != cp.TaskID || receipt.ArtifactSHA256 != cp.ArtifactSHA256 {
		return d.block(cp, "TASK_ORACLE_FAILED", errors.New("task receipt did not satisfy task oracle"))
	}
	cp.State = StateCompleted
	cp.SideEffect = SideEffectCompleted
	cp.NextRetryAt = nil
	cp.LastErrorKind = ""
	return cp, d.Store.Save(cp)
}

func (d *Dispatcher) reconcileLater(cp Checkpoint, now time.Time, kind string, cause error, countAttempt bool) (Checkpoint, error) {
	if countAttempt {
		cp.Attempts++
	}
	if cp.Attempts >= cp.RetryBudget {
		return d.block(cp, "RETRY_BUDGET_EXHAUSTED", cause)
	}
	next := now.Add(d.retryDelay()).UTC()
	cp.State = StateDeliveryUncertain
	cp.NextRetryAt = &next
	cp.LastErrorKind = kind
	if err := d.Store.Save(cp); err != nil {
		return cp, err
	}
	return cp, cause
}

func (d *Dispatcher) scheduleSendRetry(cp Checkpoint, now time.Time, kind string, cause error) (Checkpoint, error) {
	if cp.Attempts >= cp.RetryBudget {
		return d.block(cp, "RETRY_BUDGET_EXHAUSTED", cause)
	}
	next := now.Add(d.retryDelay()).UTC()
	cp.State = StateWaitingRetry
	cp.NextRetryAt = &next
	cp.LastErrorKind = kind
	if err := d.Store.Save(cp); err != nil {
		return cp, err
	}
	return cp, cause
}

func (d *Dispatcher) block(cp Checkpoint, kind string, cause error) (Checkpoint, error) {
	cp.State = StateBlocked
	cp.NextRetryAt = nil
	cp.LastErrorKind = kind
	if err := d.Store.Save(cp); err != nil {
		return cp, err
	}
	return cp, cause
}

func (d *Dispatcher) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func (d *Dispatcher) retryDelay() time.Duration {
	if d.RetryDelay > 0 {
		return d.RetryDelay
	}
	return time.Minute
}
