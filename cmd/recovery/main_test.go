package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

var binaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "recovery-cli-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	bin := filepath.Join(tmpDir, "recovery-test-bin")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build recovery binary: %v\nOutput: %s\n", err, string(out))
		os.Exit(1)
	}

	binaryPath = bin
	os.Exit(m.Run())
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("unexpected execution failure: %v", err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

func writeTempCheckpoint(t *testing.T, cp recovery.Checkpoint) string {
	t.Helper()
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		t.Fatalf("marshal checkpoint: %v", err)
	}
	tmpFile, err := os.CreateTemp("", "cp-test-*.json")
	if err != nil {
		t.Fatalf("create temp checkpoint: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(tmpFile.Name())
	})
	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("write checkpoint data: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp checkpoint: %v", err)
	}
	return tmpFile.Name()
}

func TestExitCodeCompletedCheckpoint(t *testing.T) {
	cp := recovery.Checkpoint{
		Version:         1,
		TaskID:          "task-completed-001",
		DeliveryID:      "del-001",
		SessionID:       "sess-001",
		ArtifactPath:    "go.mod",
		ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
		NextAction:      "none",
		State:           recovery.StateCompleted,
		SideEffect:      recovery.SideEffectCompleted,
		RetryBudget:     3,
		Attempts:        1,
		LastObservedUTC: time.Now().UTC(),
	}
	path := writeTempCheckpoint(t, cp)

	stdout, stderr, code := runCLI(t, "run", "-checkpoint", path)
	if code != ExitCodeSuccess {
		t.Fatalf("expected exit code %d, got %d. stderr: %s", ExitCodeSuccess, code, stderr)
	}
	if !strings.Contains(stdout, "State:             COMPLETED") {
		t.Fatalf("expected stdout to contain COMPLETED, got: %s", stdout)
	}
}

func TestExitCodeBlockedCheckpoint(t *testing.T) {
	cp := recovery.Checkpoint{
		Version:         1,
		TaskID:          "task-blocked-001",
		DeliveryID:      "del-002",
		SessionID:       "sess-002",
		ArtifactPath:    "go.mod",
		ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
		NextAction:      "retry",
		State:           recovery.StateBlocked,
		SideEffect:      recovery.SideEffectPending,
		RetryBudget:     5,
		Attempts:        5,
		LastErrorKind:   "BLOCKED_PROFILE_ELIGIBILITY_GPT55",
		LastObservedUTC: time.Now().UTC(),
	}
	path := writeTempCheckpoint(t, cp)

	stdout, stderr, code := runCLI(t, "run", "-checkpoint", path)
	if code != ExitCodeTerminalErr {
		t.Fatalf("expected exit code %d for BLOCKED, got %d. stderr: %s, stdout: %s", ExitCodeTerminalErr, code, stderr, stdout)
	}
	if !strings.Contains(stdout, "State:             BLOCKED") {
		t.Fatalf("expected stdout to contain BLOCKED, got: %s", stdout)
	}
	if !strings.Contains(stderr, "BLOCKED_PROFILE_ELIGIBILITY_GPT55") {
		t.Fatalf("expected stderr to contain error kind, got: %s", stderr)
	}
}

func TestExitCodeCancelledCheckpoints(t *testing.T) {
	states := []recovery.TaskState{
		recovery.StateCancelled,
		recovery.StateCancelUnconfirmed,
	}

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			cp := recovery.Checkpoint{
				Version:         1,
				TaskID:          "task-cancelled-001",
				DeliveryID:      "del-003",
				SessionID:       "sess-003",
				ArtifactPath:    "go.mod",
				ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
				NextAction:      "stop",
				State:           state,
				SideEffect:      recovery.SideEffectPending,
				RetryBudget:     3,
				Attempts:        1,
				LastErrorKind:   "HUMAN_STOP",
				LastObservedUTC: time.Now().UTC(),
			}
			path := writeTempCheckpoint(t, cp)

			stdout, stderr, code := runCLI(t, "run", "-checkpoint", path)
			if code != ExitCodeTerminalErr {
				t.Fatalf("expected exit code %d for %s, got %d. stderr: %s, stdout: %s", ExitCodeTerminalErr, state, code, stderr, stdout)
			}
			if !strings.Contains(stdout, string(state)) {
				t.Fatalf("expected stdout to contain %s, got: %s", state, stdout)
			}
		})
	}
}

func TestExitCodeTimeoutNonTerminal(t *testing.T) {
	repoRoot := findRepoRoot(t)
	obs, err := recovery.GitArtifactReader{Root: repoRoot}.Observe(context.Background(), "go.mod")
	if err != nil {
		t.Fatalf("observe artifact: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/conversation") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"controller": "running",
				"turns": []map[string]any{
					{
						"id":    "turn-001",
						"state": "running",
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"session": map[string]any{
				"id":           "sess-004",
				"isTerminated": false,
			},
		})
	}))
	defer server.Close()

	cp := recovery.Checkpoint{
		Version:         1,
		TaskID:          "task-timeout-001",
		DeliveryID:      "del-004",
		SessionID:       "sess-004",
		AOTurnID:        "turn-001",
		ArtifactPath:    "go.mod",
		ArtifactSHA256:  obs.SHA256,
		GitHead:         obs.GitHead,
		NextAction:      "retry",
		State:           recovery.StateDelivered,
		SideEffect:      recovery.SideEffectPending,
		RetryBudget:     10,
		Attempts:        1,
		LastErrorKind:   "",
		LastObservedUTC: time.Now().UTC(),
	}
	path := writeTempCheckpoint(t, cp)

	stdout, stderr, code := runCLI(t, "run", "-checkpoint", path, "-workspace", repoRoot, "-ao-url", server.URL, "-timeout", "1500ms", "-poll", "50ms")
	if code != ExitCodeTimeout {
		t.Fatalf("expected exit code %d for timeout, got %d. stderr: %s, stdout: %s", ExitCodeTimeout, code, stderr, stdout)
	}
	if !strings.Contains(stderr, "timed out") && !strings.Contains(stderr, "context deadline exceeded") {
		t.Fatalf("expected stderr to indicate timeout, got: %s", stderr)
	}
}

func TestExitCodeUsageAndCancelRejection(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantSubstr string
	}{
		{
			name:       "no args",
			args:       []string{},
			wantSubstr: "usage: recovery <run|status>",
		},
		{
			name:       "cancel command rejected",
			args:       []string{"cancel"},
			wantSubstr: "cancel command is not supported",
		},
		{
			name:       "unknown command rejected",
			args:       []string{"unknown-subcommand"},
			wantSubstr: "unsupported command",
		},
		{
			name:       "status missing checkpoint",
			args:       []string{"status"},
			wantSubstr: "-checkpoint is required",
		},
		{
			name:       "run missing checkpoint",
			args:       []string{"run"},
			wantSubstr: "-checkpoint is required",
		},
		{
			name:       "status checkpoint file not found",
			args:       []string{"status", "-checkpoint", "nonexistent-cp-file-xyz.json"},
			wantSubstr: "load checkpoint",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runCLI(t, tc.args...)
			if code != ExitCodeUsageError {
				t.Fatalf("expected exit code %d for usage/cancel rejection, got %d. stderr: %s", ExitCodeUsageError, code, stderr)
			}
			if !strings.Contains(stderr, tc.wantSubstr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantSubstr, stderr)
			}
		})
	}
}

func TestExitCodeStatusCommand(t *testing.T) {
	now := time.Now().UTC()
	cp := recovery.Checkpoint{
		Version:         1,
		TaskID:          "task-status-001",
		DeliveryID:      "del-status-001",
		SessionID:       "sess-status-001",
		ArtifactPath:    "go.mod",
		ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
		NextAction:      "observe",
		State:           recovery.StateWaitingRetry,
		SideEffect:      recovery.SideEffectPending,
		RetryBudget:     5,
		Attempts:        2,
		LastErrorKind:   "PROVIDER_RATE_LIMITED",
		LastObservedUTC: now,
	}
	path := writeTempCheckpoint(t, cp)

	// Test text status
	stdout, stderr, code := runCLI(t, "status", "-checkpoint", path)
	if code != ExitCodeSuccess {
		t.Fatalf("expected exit code %d for status, got %d. stderr: %s", ExitCodeSuccess, code, stderr)
	}
	if !strings.Contains(stdout, "Task ID:           task-status-001") {
		t.Fatalf("expected Task ID in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "State:             WAITING_RETRY") {
		t.Fatalf("expected State in stdout, got: %s", stdout)
	}

	// Test JSON status
	stdoutJSON, stderrJSON, codeJSON := runCLI(t, "status", "-checkpoint", path, "-json")
	if codeJSON != ExitCodeSuccess {
		t.Fatalf("expected exit code %d for status -json, got %d. stderr: %s", ExitCodeSuccess, codeJSON, stderrJSON)
	}
	var report recovery.StatusReport
	if err := json.Unmarshal([]byte(stdoutJSON), &report); err != nil {
		t.Fatalf("unmarshal JSON status output: %v, raw: %s", err, stdoutJSON)
	}
	if report.TaskID != "task-status-001" || report.State != recovery.StateWaitingRetry {
		t.Fatalf("unexpected report content: %+v", report)
	}
}
