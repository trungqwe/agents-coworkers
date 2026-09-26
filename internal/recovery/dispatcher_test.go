package recovery

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryAO struct {
	mu            sync.Mutex
	obs           Observation
	observeErr    error
	sendErr       error
	recoverState  DeliveryState
	sendTurnID    string
	recoverTurnID string
	recoverErr    error
	preflightErr  error
	stopErr       error
	onSend        func()
	onStop        func(string, string)
	sends         []string
	recovers      []string
	stops         []string
}

func (a *memoryAO) Observe(context.Context, string, string) (Observation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.obs, a.observeErr
}
func (a *memoryAO) Send(_ context.Context, _, deliveryID, _ string) (DeliveryReceipt, error) {
	if a.onSend != nil {
		a.onSend()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sends = append(a.sends, deliveryID)
	return DeliveryReceipt{State: DeliveryAccepted, TurnID: a.sendTurnID}, a.sendErr
}
func (a *memoryAO) Preflight(context.Context, SessionExpectation) error { return a.preflightErr }
func (a *memoryAO) Stop(_ context.Context, sessionID, turnID string) error {
	a.mu.Lock()
	a.stops = append(a.stops, sessionID+":"+turnID)
	onStop := a.onStop
	err := a.stopErr
	a.mu.Unlock()
	if onStop != nil {
		onStop(sessionID, turnID)
	}
	return err
}
func (a *memoryAO) RecoverOnly(_ context.Context, _, deliveryID string) (DeliveryReceipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.recovers = append(a.recovers, deliveryID)
	return DeliveryReceipt{State: a.recoverState, TurnID: a.recoverTurnID}, a.recoverErr
}

func (a *memoryAO) counts() (int, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sends), len(a.recovers)
}

type blockingStore struct {
	Store
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (s *blockingStore) Load() (Checkpoint, error) {
	s.once.Do(func() {
		close(s.entered)
		<-s.release
	})
	return s.Store.Load()
}

type fixedArtifact struct{ hash, head string }

func (f fixedArtifact) Observe(context.Context, string) (ArtifactObservation, error) {
	return ArtifactObservation{SHA256: f.hash, GitHead: f.head, Branch: "ao/session-1/root"}, nil
}

func testCheckpoint() Checkpoint {
	return Checkpoint{Version: 1, TaskID: "fixture-1", DeliveryID: "delivery-1", SessionID: "session-1", RunOwner: "run-1", SessionExclusive: true, ArtifactPath: "artifact.txt", ArtifactSHA256: "abc", GitHead: "head-1", NextAction: "read only", State: StatePending, SideEffect: SideEffectPending, RetryBudget: 3}
}

func fileStore(t *testing.T, cp Checkpoint) FileStore {
	t.Helper()
	s := FileStore{Path: filepath.Join(t.TempDir(), "checkpoint.json")}
	if err := s.Save(cp); err != nil {
		t.Fatal(err)
	}
	return s
}

func dispatcher(store Store, ao AO, now time.Time) *Dispatcher {
	return &Dispatcher{Store: store, AO: ao, Artifacts: fixedArtifact{hash: "abc", head: "head-1"}, Now: func() time.Time { return now }, RetryDelay: time.Minute}
}

func TestProviderFailureSchedulesBoundedRetry(t *testing.T) {
	cp := testCheckpoint()
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}, sendErr: ErrProviderUnavailable}
	now := time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)
	got, err := dispatcher(store, ao, now).Step(context.Background())
	if !errors.Is(err, ErrProviderUnavailable) || got.State != StateDeliveryUncertain || got.Attempts != 1 || got.NextRetryAt == nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if len(ao.sends) != 1 {
		t.Fatalf("sends=%v", ao.sends)
	}
}

func TestSendWritesUncertainCheckpointBeforeProviderIO(t *testing.T) {
	store := fileStore(t, testCheckpoint())
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}, sendTurnID: "turn-1"}
	ao.onSend = func() {
		cp, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		if cp.State != StateDeliveryUncertain || cp.LastErrorKind != "SEND_IN_FLIGHT" || cp.Attempts != 1 {
			t.Fatalf("checkpoint before send=%+v", cp)
		}
	}
	got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
	if err != nil || got.State != StateDelivered || got.AOTurnID != "turn-1" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestUncertainRecoveryBacksOffAcrossRestartAndNeverResends(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	cp.Attempts = 1
	cp.RetryBudget = 4
	cp.RetryBudget = 4
	now := time.Date(2026, 9, 25, 2, 0, 0, 0, time.UTC)
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryReserved}, recoverErr: ErrProviderUnavailable}
	d := dispatcher(store, ao, now)
	got, err := d.Step(context.Background())
	if !errors.Is(err, ErrProviderUnavailable) || got.State != StateDeliveryUncertain || got.NextRetryAt == nil {
		t.Fatalf("first reconciliation got=%+v err=%v", got, err)
	}
	deadline := *got.NextRetryAt

	// Simulate dispatcher restart: before deadline, neither send nor recover runs.
	d = dispatcher(store, ao, deadline.Add(-time.Second))
	got, err = d.Step(context.Background())
	if err != nil || got.State != StateDeliveryUncertain {
		t.Fatalf("before deadline got=%+v err=%v", got, err)
	}
	sends, recovers := ao.counts()
	if sends != 0 || recovers != 1 {
		t.Fatalf("before deadline sends=%d recovers=%d", sends, recovers)
	}

	// At the deadline only recover-only is retried; ordinary send remains forbidden.
	d = dispatcher(store, ao, deadline)
	got, err = d.Step(context.Background())
	if !errors.Is(err, ErrProviderUnavailable) || got.State != StateDeliveryUncertain {
		t.Fatalf("at deadline got=%+v err=%v", got, err)
	}
	sends, recovers = ao.counts()
	if sends != 0 || recovers != 2 {
		t.Fatalf("at deadline sends=%d recovers=%d", sends, recovers)
	}

	// Exhausting reconciliation budget blocks terminally; it still never resends.
	d = dispatcher(store, ao, (*got.NextRetryAt))
	got, err = d.Step(context.Background())
	if !errors.Is(err, ErrProviderUnavailable) || got.State != StateBlocked || got.LastErrorKind != "RETRY_BUDGET_EXHAUSTED" {
		t.Fatalf("at exhaustion got=%+v err=%v", got, err)
	}
	sends, recovers = ao.counts()
	if sends != 0 || recovers != 3 {
		t.Fatalf("at exhaustion sends=%d recovers=%d", sends, recovers)
	}
}

func TestRecoverOnlyInconclusiveStaysUncertain(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}, recoverState: DeliveryNotFound}
	got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
	if !errors.Is(err, ErrDeliveryUncertain) || got.State != StateDeliveryUncertain || got.NextRetryAt == nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	sends, recovers := ao.counts()
	if sends != 0 || recovers != 1 {
		t.Fatalf("sends=%d recovers=%d", sends, recovers)
	}
}

func TestRestartReadsCheckpointAndWaitsUntilNextRetry(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateWaitingRetry
	cp.Attempts = 1
	next := time.Date(2026, 9, 25, 1, 5, 0, 0, time.UTC)
	cp.NextRetryAt = &next
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
	got, err := dispatcher(store, ao, next.Add(-time.Second)).Step(context.Background())
	if err != nil || got.Attempts != 1 || len(ao.sends) != 0 {
		t.Fatalf("before retry got=%+v sends=%v err=%v", got, ao.sends, err)
	}
	got, err = dispatcher(store, ao, next).Step(context.Background())
	if err != nil || got.State != StateDelivered || got.Attempts != 2 || len(ao.sends) != 1 {
		t.Fatalf("at retry got=%+v sends=%v err=%v", got, ao.sends, err)
	}
}

func TestUncertainDeliveryUsesRecoverOnlyWithSameID(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}, recoverState: DeliveryAccepted}
	got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
	if err != nil || got.State != StateDelivered || len(ao.sends) != 0 || len(ao.recovers) != 1 || ao.recovers[0] != cp.DeliveryID {
		t.Fatalf("got=%+v sends=%v recovers=%v err=%v", got, ao.sends, ao.recovers, err)
	}
}

func TestCompletedTaskNeverSendsAgain(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateCompleted
	cp.SideEffect = SideEffectCompleted
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
	got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
	if err != nil || got.State != StateCompleted || len(ao.sends) != 0 || len(ao.recovers) != 0 {
		t.Fatalf("got=%+v sends=%v recovers=%v err=%v", got, ao.sends, ao.recovers, err)
	}
}

func TestAllTerminalStatesRemainTerminalAcrossRestart(t *testing.T) {
	for _, state := range []TaskState{StateBlocked, StateCancelled, StateCancelUnconfirmed, StateCompleted} {
		t.Run(string(state), func(t *testing.T) {
			cp := testCheckpoint()
			cp.State = state
			if state == StateCompleted {
				cp.SideEffect = SideEffectCompleted
			}
			store := fileStore(t, cp)
			ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
			for i := 0; i < 2; i++ {
				got, err := dispatcher(store, ao, time.Now().Add(time.Duration(i)*time.Hour)).Step(context.Background())
				if err != nil || got.State != state {
					t.Fatalf("restart %d got=%+v err=%v", i, got, err)
				}
			}
			sends, recovers := ao.counts()
			if sends != 0 || recovers != 0 {
				t.Fatalf("sends=%d recovers=%d", sends, recovers)
			}
		})
	}
}

func TestDeliveredTaskWithMissingReceiptBlocksInsteadOfReplaying(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDelivered
	cp.Attempts = 1
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
	got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
	if err == nil || got.State != StateBlocked || got.LastErrorKind != "DELIVERY_RECEIPT_LOST" || len(ao.sends) != 0 {
		t.Fatalf("got=%+v sends=%v err=%v", got, ao.sends, err)
	}

	// BLOCKED is terminal across restart, even if AO later reports no receipt.
	d := dispatcher(store, ao, time.Now().Add(time.Hour))
	got, err = d.Step(context.Background())
	sends, recovers := ao.counts()
	if err != nil || got.State != StateBlocked || sends != 0 || recovers != 0 {
		t.Fatalf("restart got=%+v sends=%d recovers=%d err=%v", got, sends, recovers, err)
	}
}

func TestDeliveredObserveErrorThenNotFoundNeverResendsAcrossRestart(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDelivered
	cp.AOTurnID = "turn-1"
	cp.Attempts = 1
	store := fileStore(t, cp)
	now := time.Date(2026, 9, 25, 4, 0, 0, 0, time.UTC)
	ao := &memoryAO{observeErr: errors.New("temporary observe failure")}
	got, err := dispatcher(store, ao, now).Step(context.Background())
	if err == nil || got.State != StateDelivered || got.NextRetryAt == nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	ao.observeErr = nil
	ao.obs = Observation{SessionExists: true, Delivery: DeliveryNotFound}
	got, err = dispatcher(store, ao, *got.NextRetryAt).Step(context.Background())
	sends, _ := ao.counts()
	if err == nil || got.State != StateBlocked || sends != 0 {
		t.Fatalf("got=%+v sends=%d err=%v", got, sends, err)
	}
}

func TestUncertainObserveErrorThenNotFoundUsesRecoverOnlyAcrossRestart(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	cp.Attempts = 1
	cp.RetryBudget = 4
	store := fileStore(t, cp)
	now := time.Date(2026, 9, 25, 5, 0, 0, 0, time.UTC)
	ao := &memoryAO{observeErr: errors.New("temporary observe failure")}
	got, err := dispatcher(store, ao, now).Step(context.Background())
	if err == nil || got.State != StateDeliveryUncertain || got.NextRetryAt == nil {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	ao.observeErr = nil
	ao.obs = Observation{SessionExists: true, Delivery: DeliveryNotFound}
	ao.recoverState = DeliveryNotFound
	got, err = dispatcher(store, ao, *got.NextRetryAt).Step(context.Background())
	sends, recovers := ao.counts()
	if err == nil || got.State != StateDeliveryUncertain || sends != 0 || recovers != 1 {
		t.Fatalf("got=%+v sends=%d recovers=%d err=%v", got, sends, recovers, err)
	}
}

func TestSteeredOutcomeBlocksAsSessionContamination(t *testing.T) {
	store := fileStore(t, testCheckpoint())
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}, sendErr: ErrSessionContaminated}
	got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
	if !errors.Is(err, ErrSessionContaminated) || got.State != StateBlocked || got.LastErrorKind != "SESSION_CONTAMINATED" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestRunOwnedLeaseAllowsOnlyOneSender(t *testing.T) {
	cp := testCheckpoint()
	cp.RunOwner = "run-1"
	store := fileStore(t, cp)
	entered, releaseSend := make(chan struct{}), make(chan struct{})
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}, sendTurnID: "turn-1"}
	ao.onSend = func() { close(entered); <-releaseSend }
	leaseRoot := t.TempDir()
	d1 := dispatcher(store, ao, time.Now())
	lease1, err := NewFileLease(leaseRoot, "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	d1.Lease = lease1
	d2 := dispatcher(store, ao, time.Now())
	lease2, err := NewFileLease(leaseRoot, "owner-2")
	if err != nil {
		t.Fatal(err)
	}
	d2.Lease = lease2
	done := make(chan error, 1)
	go func() { _, err := d1.Step(context.Background()); done <- err }()
	<-entered
	if _, err := d2.Step(context.Background()); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("second owner err=%v", err)
	}
	close(releaseSend)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	sends, _ := ao.counts()
	if sends != 1 {
		t.Fatalf("sends=%d", sends)
	}
}

func TestAOTurnCompletionRequiresAcceptedTaskReceipt(t *testing.T) {
	tests := []struct {
		name    string
		receipt *TaskReceipt
		state   TaskState
		kind    string
	}{
		{name: "missing", state: StateBlocked, kind: "TASK_RECEIPT_MISSING"},
		{name: "wrong artifact", receipt: &TaskReceipt{TaskID: "fixture-1", ArtifactSHA256: "wrong", Accepted: true}, state: StateBlocked, kind: "TASK_ORACLE_FAILED"},
		{name: "accepted", receipt: &TaskReceipt{TaskID: "fixture-1", ArtifactSHA256: "abc", Accepted: true}, state: StateCompleted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := fileStore(t, testCheckpoint())
			ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryCompleted, Receipt: tt.receipt}}
			got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
			if got.State != tt.state || got.LastErrorKind != tt.kind {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			if tt.state == StateCompleted && (err != nil || got.SideEffect != SideEffectCompleted) {
				t.Fatalf("accepted receipt got=%+v err=%v", got, err)
			}
		})
	}
}

func TestExplicitHumanStopIsTerminalCancellation(t *testing.T) {
	store := fileStore(t, testCheckpoint())
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
	d := dispatcher(store, ao, time.Now())
	got, err := d.Cancel(context.Background(), time.Now())
	if err != nil || got.State != StateCancelled || got.LastErrorKind != "HUMAN_STOP" || len(ao.sends) != 0 {
		t.Fatalf("got=%+v sends=%v err=%v", got, ao.sends, err)
	}

	// A fresh dispatcher process observes the durable cancellation and does no I/O.
	ao.obs.HumanStopped = false
	d.Now = func() time.Time { return time.Now().Add(time.Hour) }
	got, err = d.Step(context.Background())
	if err != nil || got.State != StateCancelled || len(ao.sends) != 0 {
		t.Fatalf("restart got=%+v sends=%v err=%v", got, ao.sends, err)
	}
}

func TestAOObservedHumanStopIsTerminalAcrossRestart(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDelivered
	cp.AOTurnID = "turn-1"
	store := fileStore(t, cp)
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryStopped}}
	got, err := dispatcher(store, ao, time.Now()).Step(context.Background())
	if err != nil || got.State != StateCancelled {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	ao.obs = Observation{SessionExists: true, Delivery: DeliveryNotFound}
	got, err = dispatcher(store, ao, time.Now().Add(time.Hour)).Step(context.Background())
	sends, recovers := ao.counts()
	if err != nil || got.State != StateCancelled || sends != 0 || recovers != 0 {
		t.Fatalf("got=%+v sends=%d recovers=%d err=%v", got, sends, recovers, err)
	}
}

func TestCancelPersistsBeforeAOInterruptFailure(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDelivered
	cp.AOTurnID = "turn-1"
	store := fileStore(t, cp)
	ao := &memoryAO{stopErr: errors.New("interrupt response lost")}
	got, err := dispatcher(store, ao, time.Now()).Cancel(context.Background(), time.Now())
	if err == nil || got.State != StateCancelUnconfirmed {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	got, err = dispatcher(store, ao, time.Now().Add(time.Hour)).Step(context.Background())
	sends, recovers := ao.counts()
	if err != nil || got.State != StateCancelUnconfirmed || sends != 0 || recovers != 0 {
		t.Fatalf("got=%+v sends=%d recovers=%d err=%v", got, sends, recovers, err)
	}
}

func TestLostAcceptanceThenCancelRecoversExactTurnAndNeverResends(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	cp.Attempts = 1
	store := fileStore(t, cp)
	ao := &memoryAO{recoverState: DeliveryAccepted, recoverTurnID: "turn-owned"}
	ao.onStop = func(_, _ string) {
		ao.mu.Lock()
		ao.obs = Observation{SessionExists: true, Delivery: DeliveryStopped}
		ao.mu.Unlock()
	}
	got, err := dispatcher(store, ao, time.Now()).Cancel(context.Background(), time.Now())
	if err != nil || got.State != StateCancelled || got.AOTurnID != "turn-owned" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	sends, recovers := ao.counts()
	if sends != 0 || recovers != 1 || len(ao.stops) != 1 || ao.stops[0] != "session-1:turn-owned" {
		t.Fatalf("sends=%d recovers=%d stops=%v", sends, recovers, ao.stops)
	}
	got, err = dispatcher(store, ao, time.Now().Add(time.Hour)).Step(context.Background())
	if err != nil || got.State != StateCancelled {
		t.Fatalf("restart got=%+v err=%v", got, err)
	}
	sends, _ = ao.counts()
	if sends != 0 {
		t.Fatalf("ordinary sends=%d", sends)
	}
}

func TestLostAcceptanceCancelInterruptResponseLossCanConfirmByObservation(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	cp.Attempts = 1
	store := fileStore(t, cp)
	ao := &memoryAO{recoverState: DeliveryAccepted, recoverTurnID: "turn-owned", stopErr: errors.New("interrupt response lost")}
	ao.onStop = func(_, _ string) {
		ao.mu.Lock()
		ao.obs = Observation{SessionExists: true, Delivery: DeliveryStopped}
		ao.mu.Unlock()
	}
	got, err := dispatcher(store, ao, time.Now()).Cancel(context.Background(), time.Now())
	if err != nil || got.State != StateCancelled {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	sends, recovers := ao.counts()
	if sends != 0 || recovers != 1 || len(ao.stops) != 1 {
		t.Fatalf("sends=%d recovers=%d stops=%v", sends, recovers, ao.stops)
	}
}

func TestLostAcceptanceCancelUnconfirmedSurvivesRestartWithoutStoppingUnownedTurn(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	cp.Attempts = 1
	store := fileStore(t, cp)
	ao := &memoryAO{recoverState: DeliveryNotFound}
	got, err := dispatcher(store, ao, time.Now()).Cancel(context.Background(), time.Now())
	if !errors.Is(err, ErrDeliveryUncertain) || got.State != StateCancelUnconfirmed || len(ao.stops) != 0 {
		t.Fatalf("got=%+v stops=%v err=%v", got, ao.stops, err)
	}
	got, err = dispatcher(store, ao, time.Now().Add(time.Hour)).Step(context.Background())
	sends, _ := ao.counts()
	if err != nil || got.State != StateCancelUnconfirmed || sends != 0 || len(ao.stops) != 0 {
		t.Fatalf("restart got=%+v sends=%d stops=%v err=%v", got, sends, ao.stops, err)
	}
}

func TestCancelReconciliationAfterRestartStopsRecoveredOwnedTurn(t *testing.T) {
	cp := testCheckpoint()
	cp.State = StateDeliveryUncertain
	cp.Attempts = 1
	store := fileStore(t, cp)
	ao := &memoryAO{recoverState: DeliveryNotFound}
	got, err := dispatcher(store, ao, time.Now()).Cancel(context.Background(), time.Now())
	if !errors.Is(err, ErrDeliveryUncertain) || got.State != StateCancelUnconfirmed {
		t.Fatalf("first got=%+v err=%v", got, err)
	}
	ao.recoverState = DeliveryAccepted
	ao.recoverTurnID = "turn-owned"
	ao.onStop = func(_, _ string) {
		ao.mu.Lock()
		ao.obs = Observation{SessionExists: true, Delivery: DeliveryStopped}
		ao.mu.Unlock()
	}
	got, err = dispatcher(store, ao, time.Now().Add(time.Minute)).Cancel(context.Background(), time.Now().Add(time.Minute))
	if err != nil || got.State != StateCancelled || got.AOTurnID != "turn-owned" {
		t.Fatalf("restart cancel got=%+v err=%v", got, err)
	}
	sends, recovers := ao.counts()
	if sends != 0 || recovers != 2 || len(ao.stops) != 1 || ao.stops[0] != "session-1:turn-owned" {
		t.Fatalf("sends=%d recovers=%d stops=%v", sends, recovers, ao.stops)
	}
}

func TestCancelClassifiesSettledTargetTurnWithoutInterrupt(t *testing.T) {
	tests := []struct {
		name string
		obs  Observation
		want TaskState
		kind string
	}{
		{name: "interrupted", obs: Observation{SessionExists: true, Delivery: DeliveryStopped}, want: StateCancelled, kind: "HUMAN_STOP"},
		{name: "completed_valid_receipt", obs: Observation{SessionExists: true, Delivery: DeliveryCompleted, Receipt: &TaskReceipt{TaskID: "fixture-1", ArtifactSHA256: "abc", Accepted: true}}, want: StateCompleted},
		{name: "completed_missing_receipt", obs: Observation{SessionExists: true, Delivery: DeliveryCompleted}, want: StateCancelUnconfirmed, kind: "COMPLETED_OUTCOME_UNVERIFIED"},
		{name: "failed", obs: Observation{SessionExists: true, Delivery: DeliveryRejected}, want: StateBlocked, kind: "DELIVERY_FAILED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := testCheckpoint()
			cp.State = StateDelivered
			cp.AOTurnID = "turn-owned"
			store := fileStore(t, cp)
			ao := &memoryAO{obs: tt.obs}
			got, err := dispatcher(store, ao, time.Now()).Cancel(context.Background(), time.Now())
			if err != nil || got.State != tt.want || got.LastErrorKind != tt.kind || len(ao.stops) != 0 {
				t.Fatalf("got=%+v stops=%v err=%v", got, ao.stops, err)
			}
			got, err = dispatcher(store, ao, time.Now().Add(time.Hour)).Step(context.Background())
			sends, recovers := ao.counts()
			if err != nil || got.State != tt.want || sends != 0 || recovers != 0 || len(ao.stops) != 0 {
				t.Fatalf("restart got=%+v sends=%d recovers=%d stops=%v err=%v", got, sends, recovers, ao.stops, err)
			}
		})
	}
}

func TestCancelWinsSerializedRaceAndStepCannotSend(t *testing.T) {
	base := fileStore(t, testCheckpoint())
	store := &blockingStore{Store: base, entered: make(chan struct{}), release: make(chan struct{})}
	ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
	d := dispatcher(store, ao, time.Now())
	cancelDone := make(chan error, 1)
	go func() {
		_, err := d.Cancel(context.Background(), time.Now())
		cancelDone <- err
	}()
	<-store.entered // Cancel owns the dispatcher lock and is inside Load.
	stepDone := make(chan error, 1)
	go func() {
		_, err := d.Step(context.Background())
		stepDone <- err
	}()
	close(store.release)
	if err := <-cancelDone; err != nil {
		t.Fatal(err)
	}
	if err := <-stepDone; err != nil {
		t.Fatal(err)
	}
	cp, err := base.Load()
	if err != nil {
		t.Fatal(err)
	}
	sends, recovers := ao.counts()
	if cp.State != StateCancelled || sends != 0 || recovers != 0 {
		t.Fatalf("checkpoint=%+v sends=%d recovers=%d", cp, sends, recovers)
	}
}

func TestGitArtifactReaderBindsHashAndHead(t *testing.T) {
	repoDir := t.TempDir()

	const fixtureBranch = "fixture-test-branch"
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v, output: %s", args, err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test User")
	runGit("config", "user.email", "test@example.com")
	runGit("checkout", "-b", fixtureBranch)

	artifactName := "artifact.txt"
	artifactContent := []byte("deterministic artifact content for recovery test\n")
	artifactPath := filepath.Join(repoDir, artifactName)
	if err := os.WriteFile(artifactPath, artifactContent, 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	runGit("add", artifactName)
	runGit("commit", "-m", "commit artifact fixture")

	h := sha256.Sum256(artifactContent)
	expectedSHA := fmt.Sprintf("%x", h)

	headOut, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("read expected git head: %v", err)
	}
	expectedHead := strings.TrimSpace(string(headOut))

	branchOut, err := exec.Command("git", "-C", repoDir, "branch", "--show-current").Output()
	if err != nil {
		t.Fatalf("read expected git branch: %v", err)
	}
	expectedBranch := strings.TrimSpace(string(branchOut))
	if expectedBranch != fixtureBranch || expectedBranch == "" {
		t.Fatalf("expected branch mismatch: got %q, want %q", expectedBranch, fixtureBranch)
	}

	reader := GitArtifactReader{Root: repoDir}
	got, err := reader.Observe(context.Background(), artifactName)
	if err != nil {
		t.Fatalf("observe failed: %v", err)
	}

	if got.SHA256 != expectedSHA {
		t.Fatalf("SHA256 mismatch: got %s, want %s", got.SHA256, expectedSHA)
	}
	if got.GitHead != expectedHead {
		t.Fatalf("GitHead mismatch: got %s, want %s", got.GitHead, expectedHead)
	}
	if got.Branch == "" || got.Branch != expectedBranch {
		t.Fatalf("Branch mismatch: got %q, want non-empty %q", got.Branch, expectedBranch)
	}

	outside := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+"-outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	if _, err := reader.Observe(context.Background(), filepath.Join("..", filepath.Base(outside))); err == nil {
		t.Fatal("path escaping repository root was accepted")
	}
}
