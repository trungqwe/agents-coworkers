package recovery

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLiveAODeliveryRecoveryAfterAcceptedResponseLoss(t *testing.T) {
	baseURL := os.Getenv("P7D_LIVE_AO_BASE_URL")
	sessionID := os.Getenv("P7D_LIVE_AO_SESSION_ID")
	workspace := os.Getenv("P7D_LIVE_AO_WORKSPACE")
	runOwner := os.Getenv("P7D_LIVE_RUN_OWNER")
	if baseURL == "" || sessionID == "" || workspace == "" || runOwner == "" {
		t.Skip("isolated AO fixture is not configured")
	}

	artifact, err := (GitArtifactReader{Root: workspace}).Observe(context.Background(), "go.mod")
	if err != nil {
		t.Fatal(err)
	}
	deliveryID := "p7d-live-delivery-" + time.Now().UTC().Format("20060102t150405.000000000")
	taskID := "P7D-LIVE-RECOVERY-01"
	prompt := "Read go.mod only. Do not modify files. Reply with exactly one final line: TASK_RECEIPT {\"taskId\":\"" + taskID + "\",\"artifactSha256\":\"" + artifact.SHA256 + "\",\"accepted\":true}"
	cp := Checkpoint{Version: 1, TaskID: taskID, DeliveryID: deliveryID, SessionID: sessionID,
		ProjectID: "p7d-live-fixture", SessionKind: "worker", SessionHarness: "codex",
		SessionModel: "gemini-3.8-flash-high", SessionEffort: "high", SessionBranch: artifact.Branch,
		RunOwner: runOwner, SessionExclusive: true,
		ArtifactPath: "go.mod", ArtifactSHA256: artifact.SHA256, GitHead: artifact.GitHead,
		NextAction: prompt, State: StatePending, SideEffect: SideEffectPending, RetryBudget: 20}
	store := FileStore{Path: filepath.Join(t.TempDir(), "checkpoint.json")}
	if err := store.Save(cp); err != nil {
		t.Fatal(err)
	}

	baseTransport := http.DefaultTransport
	var mu sync.Mutex
	ordinary, recoveries := 0, 0
	drop := true
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		isDelivery := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/conversation/steer-or-send")
		if !isDelivery {
			return baseTransport.RoundTrip(req)
		}
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(strings.NewReader(string(body)))
		isRecovery := strings.Contains(string(body), `"recoverOnly":true`)
		mu.Lock()
		if isRecovery {
			recoveries++
		} else {
			ordinary++
		}
		shouldDrop := drop && !isRecovery
		if shouldDrop {
			drop = false
		}
		mu.Unlock()
		resp, err := baseTransport.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		if shouldDrop {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil, errors.New("simulated response loss after live AO acceptance")
		}
		return resp, nil
	})
	client, err := NewAOHTTPClient(baseURL, &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := NewFileLease(workspace, "live-wrapper-owner")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	d := &Dispatcher{Store: store, AO: client, Artifacts: GitArtifactReader{Root: workspace}, Now: func() time.Time { return now }, RetryDelay: 100 * time.Millisecond, Lease: lease}
	got, err := d.Step(context.Background())
	if err == nil || got.State != StateDeliveryUncertain {
		t.Fatalf("initial delivery got=%+v err=%v", got, err)
	}
	now = got.NextRetryAt.UTC()
	got, err = d.Step(context.Background())
	if err != nil || got.State != StateDelivered || got.AOTurnID == "" {
		t.Fatalf("recover-only got=%+v err=%v", got, err)
	}
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		now = time.Now().UTC()
		got, err = d.Step(context.Background())
		if got.State == StateCompleted {
			break
		}
		if err != nil {
			t.Fatalf("observe completion got=%+v err=%v", got, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	if got.State != StateCompleted || got.SideEffect != SideEffectCompleted {
		t.Fatalf("final checkpoint=%+v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if ordinary != 1 || recoveries != 1 {
		t.Fatalf("ordinary=%d recoveries=%d", ordinary, recoveries)
	}
}
