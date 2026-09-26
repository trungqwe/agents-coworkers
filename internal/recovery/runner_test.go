package recovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRunner_SuccessfulRunToCompletion(t *testing.T) {
	cp := testCheckpoint()
	store := fileStore(t, cp)
	ao := &memoryAO{
		obs:        Observation{SessionExists: true, Delivery: DeliveryNotFound},
		sendTurnID: "turn-1",
	}
	d := dispatcher(store, ao, time.Now())

	ao.onSend = func() {
		ao.mu.Lock()
		defer ao.mu.Unlock()
		ao.obs = Observation{
			SessionExists: true,
			Delivery:      DeliveryCompleted,
			Receipt: &TaskReceipt{
				TaskID:         "fixture-1",
				ArtifactSHA256: "abc",
				Accepted:       true,
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := Run(ctx, d, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.State != StateCompleted {
		t.Fatalf("expected state %s, got %s", StateCompleted, got.State)
	}
	if got.SideEffect != SideEffectCompleted {
		t.Fatalf("expected side effect %s, got %s", SideEffectCompleted, got.SideEffect)
	}
	if len(ao.sends) != 1 {
		t.Fatalf("expected 1 send, got %d", len(ao.sends))
	}
}

func TestRunner_TerminatingOnBlockedState(t *testing.T) {
	t.Run("retry_budget_exhausted", func(t *testing.T) {
		cp := testCheckpoint()
		cp.Attempts = 3
		cp.RetryBudget = 3
		store := fileStore(t, cp)
		ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
		d := dispatcher(store, ao, time.Now())

		got, err := Run(context.Background(), d, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("expected nil error on terminal blocked state, got: %v", err)
		}
		if got.State != StateBlocked {
			t.Fatalf("expected state %s, got %s", StateBlocked, got.State)
		}
		if got.LastErrorKind != "RETRY_BUDGET_EXHAUSTED" {
			t.Fatalf("expected last error kind RETRY_BUDGET_EXHAUSTED, got %s", got.LastErrorKind)
		}
	})

	t.Run("delivery_rejected", func(t *testing.T) {
		cp := testCheckpoint()
		store := fileStore(t, cp)
		ao := &memoryAO{
			obs: Observation{SessionExists: true, Delivery: DeliveryRejected},
		}
		d := dispatcher(store, ao, time.Now())

		got, err := Run(context.Background(), d, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("expected nil error on terminal blocked state, got: %v", err)
		}
		if got.State != StateBlocked {
			t.Fatalf("expected state %s, got %s", StateBlocked, got.State)
		}
		if got.LastErrorKind != "DELIVERY_REJECTED" {
			t.Fatalf("expected last error kind DELIVERY_REJECTED, got %s", got.LastErrorKind)
		}
	})

	t.Run("artifact_changed", func(t *testing.T) {
		cp := testCheckpoint()
		cp.ArtifactSHA256 = "expected-sha"
		store := fileStore(t, cp)
		ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
		d := &Dispatcher{
			Store:     store,
			AO:        ao,
			Artifacts: fixedArtifact{hash: "different-sha", head: "head-1"},
		}

		got, err := Run(context.Background(), d, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("expected nil error on terminal blocked state, got: %v", err)
		}
		if got.State != StateBlocked {
			t.Fatalf("expected state %s, got %s", StateBlocked, got.State)
		}
		if got.LastErrorKind != "ARTIFACT_CHANGED" {
			t.Fatalf("expected last error kind ARTIFACT_CHANGED, got %s", got.LastErrorKind)
		}
	})
}

func TestRunner_RespectsNextRetryAtBackoff_NoHotRetry(t *testing.T) {
	t.Run("delivery_uncertain_backoff", func(t *testing.T) {
		startTime := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
		currentTime := startTime

		cp := testCheckpoint()
		store := fileStore(t, cp)
		ao := &memoryAO{
			obs:     Observation{SessionExists: true, Delivery: DeliveryNotFound},
			sendErr: ErrProviderUnavailable,
		}

		retryDelay := 5 * time.Second
		d := &Dispatcher{
			Store:      store,
			AO:         ao,
			Artifacts:  fixedArtifact{hash: "abc", head: "head-1"},
			Now:        func() time.Time { return currentTime },
			RetryDelay: retryDelay,
		}

		var (
			mu         sync.Mutex
			sleepCalls []time.Duration
		)

		cfg := RunnerConfig{
			Dispatcher:   d,
			PollInterval: 10 * time.Millisecond,
			Now:          func() time.Time { return currentTime },
			Sleep: func(ctx context.Context, dur time.Duration) error {
				mu.Lock()
				sleepCalls = append(sleepCalls, dur)
				currentTime = currentTime.Add(dur)
				ao.mu.Lock()
				ao.recoverState = DeliveryAccepted
				ao.recoverTurnID = "turn-retry-1"
				ao.obs = Observation{
					SessionExists: true,
					Delivery:      DeliveryCompleted,
					Receipt: &TaskReceipt{
						TaskID:         "fixture-1",
						ArtifactSHA256: "abc",
						Accepted:       true,
					},
				}
				ao.mu.Unlock()
				mu.Unlock()
				return nil
			},
		}

		runner := NewRunner(cfg)
		got, err := runner.Run(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != StateCompleted {
			t.Fatalf("expected state %s, got %s", StateCompleted, got.State)
		}

		mu.Lock()
		defer mu.Unlock()
		if len(sleepCalls) < 1 {
			t.Fatalf("expected at least 1 sleep call for backoff, got %d", len(sleepCalls))
		}
		if sleepCalls[0] != retryDelay {
			t.Fatalf("expected first sleep of %v, got %v", retryDelay, sleepCalls[0])
		}
	})

	t.Run("waiting_retry_backoff", func(t *testing.T) {
		startTime := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
		currentTime := startTime

		cp := testCheckpoint()
		store := fileStore(t, cp)
		ao := &memoryAO{
			obs:     Observation{SessionExists: true, Delivery: DeliveryNotFound},
			sendErr: ErrRequestNotAccepted,
		}

		retryDelay := 15 * time.Second
		d := &Dispatcher{
			Store:      store,
			AO:         ao,
			Artifacts:  fixedArtifact{hash: "abc", head: "head-1"},
			Now:        func() time.Time { return currentTime },
			RetryDelay: retryDelay,
		}

		var (
			mu         sync.Mutex
			sleepCalls []time.Duration
		)

		cfg := RunnerConfig{
			Dispatcher:   d,
			PollInterval: 10 * time.Millisecond,
			Now:          func() time.Time { return currentTime },
			Sleep: func(ctx context.Context, dur time.Duration) error {
				mu.Lock()
				sleepCalls = append(sleepCalls, dur)
				currentTime = currentTime.Add(dur)
				ao.mu.Lock()
				ao.sendErr = nil
				ao.sendTurnID = "turn-retry-2"
				ao.obs = Observation{
					SessionExists: true,
					Delivery:      DeliveryCompleted,
					Receipt: &TaskReceipt{
						TaskID:         "fixture-1",
						ArtifactSHA256: "abc",
						Accepted:       true,
					},
				}
				ao.mu.Unlock()
				mu.Unlock()
				return nil
			},
		}

		runner := NewRunner(cfg)
		got, err := runner.Run(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != StateCompleted {
			t.Fatalf("expected state %s, got %s", StateCompleted, got.State)
		}

		mu.Lock()
		defer mu.Unlock()
		if len(sleepCalls) < 1 {
			t.Fatalf("expected at least 1 sleep call for backoff, got %d", len(sleepCalls))
		}
		if sleepCalls[0] != retryDelay {
			t.Fatalf("expected sleep of %v, got %v", retryDelay, sleepCalls[0])
		}
	})

	t.Run("real_timer_backoff_no_hot_retry", func(t *testing.T) {
		cp := testCheckpoint()
		store := fileStore(t, cp)
		ao := &memoryAO{
			obs:     Observation{SessionExists: true, Delivery: DeliveryNotFound},
			sendErr: ErrRequestNotAccepted,
		}

		retryDelay := 50 * time.Millisecond
		d := &Dispatcher{
			Store:      store,
			AO:         ao,
			Artifacts:  fixedArtifact{hash: "abc", head: "head-1"},
			RetryDelay: retryDelay,
		}

		// On second send attempt, succeed and complete
		stepCount := 0
		ao.onSend = func() {
			ao.mu.Lock()
			defer ao.mu.Unlock()
			stepCount++
			if stepCount >= 2 {
				ao.sendErr = nil
				ao.sendTurnID = "turn-2"
				ao.obs = Observation{
					SessionExists: true,
					Delivery:      DeliveryCompleted,
					Receipt: &TaskReceipt{
						TaskID:         "fixture-1",
						ArtifactSHA256: "abc",
						Accepted:       true,
					},
				}
			}
		}

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		got, err := Run(ctx, d, 10*time.Millisecond)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != StateCompleted {
			t.Fatalf("expected state %s, got %s", StateCompleted, got.State)
		}
		if elapsed < retryDelay {
			t.Fatalf("expected elapsed time >= %v, got %v", retryDelay, elapsed)
		}
	})
}

func TestRunner_RespectsContextCancellation(t *testing.T) {
	t.Run("cancelled_during_retry_backoff", func(t *testing.T) {
		cp := testCheckpoint()
		store := fileStore(t, cp)
		ao := &memoryAO{
			obs:     Observation{SessionExists: true, Delivery: DeliveryNotFound},
			sendErr: ErrProviderUnavailable,
		}
		d := dispatcher(store, ao, time.Now())
		d.RetryDelay = 10 * time.Minute

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		got, err := Run(ctx, d, 10*time.Millisecond)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
		if got.State != StateDeliveryUncertain {
			t.Fatalf("expected state %s, got %s", StateDeliveryUncertain, got.State)
		}
	})

	t.Run("cancelled_before_run", func(t *testing.T) {
		cp := testCheckpoint()
		store := fileStore(t, cp)
		ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
		d := dispatcher(store, ao, time.Now())

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := Run(ctx, d, 10*time.Millisecond)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
		if len(ao.sends) != 0 {
			t.Fatalf("expected 0 sends on pre-cancelled context, got %d", len(ao.sends))
		}
	})

	t.Run("cancelled_during_poll_interval", func(t *testing.T) {
		cp := testCheckpoint()
		cp.State = StateDelivered
		cp.AOTurnID = "turn-in-flight"
		store := fileStore(t, cp)
		ao := &memoryAO{
			obs: Observation{SessionExists: true, Delivery: DeliveryAccepted},
		}
		d := dispatcher(store, ao, time.Now())

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		got, err := Run(ctx, d, 10*time.Minute)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
		if got.State != StateDelivered {
			t.Fatalf("expected state %s, got %s", StateDelivered, got.State)
		}
	})
}

func TestRunner_TerminalStates(t *testing.T) {
	terminalStates := []TaskState{
		StateCompleted,
		StateBlocked,
		StateCancelled,
		StateCancelUnconfirmed,
	}

	for _, state := range terminalStates {
		t.Run(string(state), func(t *testing.T) {
			cp := testCheckpoint()
			cp.State = state
			if state == StateCompleted {
				cp.SideEffect = SideEffectCompleted
			}
			store := fileStore(t, cp)
			ao := &memoryAO{obs: Observation{SessionExists: true, Delivery: DeliveryNotFound}}
			d := dispatcher(store, ao, time.Now())

			got, err := Run(context.Background(), d, 10*time.Millisecond)
			if err != nil {
				t.Fatalf("unexpected error for terminal state %s: %v", state, err)
			}
			if got.State != state {
				t.Fatalf("expected state %s, got %s", state, got.State)
			}
			if len(ao.sends) != 0 {
				t.Fatalf("expected no sends for terminal state %s, got %d", state, len(ao.sends))
			}
		})
	}
}

func TestRunner_NilDispatcher(t *testing.T) {
	_, err := Run(context.Background(), nil, 10*time.Millisecond)
	if !errors.Is(err, ErrNilDispatcher) {
		t.Fatalf("expected ErrNilDispatcher, got: %v", err)
	}
}
