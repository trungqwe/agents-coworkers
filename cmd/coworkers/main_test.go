package main

import (
	"bytes"
	"context"
	"crypto/sha256"
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

	"github.com/trungqwe/agents-coworkers/internal/recovery"
	"github.com/trungqwe/agents-coworkers/internal/workforce/control"
)

func TestCLI_Subcommands(t *testing.T) {
	ctx := context.Background()

	// 1. No args -> exit 1
	var stdout, stderr bytes.Buffer
	code := runCLI(ctx, []string{}, &stdout, &stderr)
	if code != control.ExitCodeUsageOrUnsupported {
		t.Errorf("expected exit code 1 on empty args, got %d", code)
	}

	// 2. Unknown subcommand -> exit 1
	stdout.Reset()
	stderr.Reset()
	code = runCLI(ctx, []string{"foo"}, &stdout, &stderr)
	if code != control.ExitCodeUsageOrUnsupported {
		t.Errorf("expected exit code 1 on unknown subcommand, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Errorf("expected unknown subcommand message, got: %s", stderr.String())
	}

	// 3. coworkers run -> exit 1 with unsupported message
	stdout.Reset()
	stderr.Reset()
	code = runCLI(ctx, []string{"run"}, &stdout, &stderr)
	if code != control.ExitCodeUsageOrUnsupported {
		t.Errorf("expected exit code 1 on run, got %d", code)
	}
	if !strings.Contains(stderr.String(), "coworkers run is not supported in Slice 8A") {
		t.Errorf("expected unsupported message on run, got: %s", stderr.String())
	}

	// 4. coworkers doctor without --spec -> exit 2
	stdout.Reset()
	stderr.Reset()
	code = runCLI(ctx, []string{"doctor"}, &stdout, &stderr)
	if code != control.ExitCodeValidation {
		t.Errorf("expected exit code 2 on doctor without spec, got %d", code)
	}

	// 5. coworkers doctor with --manifest -> exit 2
	stdout.Reset()
	stderr.Reset()
	code = runCLI(ctx, []string{"doctor", "--spec", "spec.json", "--manifest", "out.json"}, &stdout, &stderr)
	if code != control.ExitCodeValidation {
		t.Errorf("expected exit code 2 on doctor with --manifest, got %d", code)
	}

	// 6. coworkers attach missing flags -> exit 2
	stdout.Reset()
	stderr.Reset()
	code = runCLI(ctx, []string{"attach"}, &stdout, &stderr)
	if code != control.ExitCodeValidation {
		t.Errorf("expected exit code 2 on attach without flags, got %d", code)
	}

	// 7. coworkers status missing --manifest -> exit 2
	stdout.Reset()
	stderr.Reset()
	code = runCLI(ctx, []string{"status"}, &stdout, &stderr)
	if code != control.ExitCodeValidation {
		t.Errorf("expected exit code 2 on status without manifest, got %d", code)
	}
}

func TestL4_DurableProcessOracle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping L4 durable process test in short mode")
	}

	tempDir := t.TempDir()
	binName := "coworkers"
	if runtime.GOOS == "windows" {
		binName = "coworkers.exe"
	}
	exePath := filepath.Join(tempDir, binName)

	// Build real binary
	buildCmd := exec.Command("go", "build", "-o", exePath, ".")
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build cmd/coworkers failed: %v, out: %s", err, string(out))
	}

	// Setup fake AO and Gateway HTTP servers
	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-smoke-host", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/smoke-sess-001" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":           "smoke-sess-001",
					"projectId":    "proj-smoke",
					"kind":         "worker",
					"harness":      "codex",
					"model":        "gpt-5",
					"branch":       "smoke-branch",
					"isTerminated": false,
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/smoke-sess-001/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "gpt-5",
					"reasoningEffort": "low",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer aoServer.Close()

	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "gpt-5"}, {"id": "gemini-2.5"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer gwServer.Close()

	// Setup Git Fixture
	fixtureDir := filepath.Join(tempDir, "git-fixture")
	productRoot := filepath.Join(fixtureDir, "product")
	worktreeDir := filepath.Join(fixtureDir, "worktree")

	_ = os.MkdirAll(productRoot, 0o755)
	_ = exec.Command("git", "init", productRoot).Run()
	_ = exec.Command("git", "-C", productRoot, "config", "user.name", "Tester").Run()
	_ = exec.Command("git", "-C", productRoot, "config", "user.email", "tester@example.com").Run()

	readmePath := filepath.Join(productRoot, "README.md")
	_ = os.WriteFile(readmePath, []byte("# Smoke Target\n"), 0o644)
	_ = exec.Command("git", "-C", productRoot, "add", ".").Run()
	_ = exec.Command("git", "-C", productRoot, "commit", "-m", "init").Run()

	shaOut, _ := exec.Command("git", "-C", productRoot, "rev-parse", "HEAD").Output()
	baselineSha := string(bytes.TrimSpace(shaOut))

	_ = exec.Command("git", "-C", productRoot, "worktree", "add", "-b", "smoke-branch", worktreeDir, baselineSha).Run()

	// Write valid RunSpec
	specPath := filepath.Join(tempDir, "run-spec.json")
	validSpecJSON := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-smoke-20261001-001",
  "description": "L4 executable smoke test run",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "smoke-branch"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [
    {"name": "git", "required": true}
  ],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "profiles": {
    "worker": {
      "model": "gpt-5",
      "reasoningEffort": "low"
    }
  }
}`, filepath.ToSlash(productRoot), filepath.ToSlash(worktreeDir), baselineSha)
	_ = os.WriteFile(specPath, []byte(validSpecJSON), 0o644)

	manifestPath := filepath.Join(tempDir, "run-manifest.json")

	runSub := func(t *testing.T, expectedExit int, args ...string) (string, string) {
		t.Helper()
		cmd := exec.Command(exePath, args...)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()
		exitCode := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				t.Fatalf("failed to execute binary: %v", err)
			}
		}
		if exitCode != expectedExit {
			t.Errorf("expected exit code %d, got %d. Stderr: %s", expectedExit, exitCode, errBuf.String())
		}
		return outBuf.String(), errBuf.String()
	}

	t.Run("01_doctor_healthy", func(t *testing.T) {
		runSub(t, 0, "doctor", "--spec", specPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL, "--json")
	})

	t.Run("02_attach_initial_create", func(t *testing.T) {
		runSub(t, 0, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", manifestPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL, "--json")
	})

	t.Run("03_attach_idempotent_second_call", func(t *testing.T) {
		runSub(t, 0, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", manifestPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL, "--json")
	})

	t.Run("04_status_valid_manifest", func(t *testing.T) {
		runSub(t, 0, "status", "--manifest", manifestPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL, "--json")
	})

	t.Run("05_run_unsupported", func(t *testing.T) {
		runSub(t, 1, "run")
	})

	t.Run("06_doctor_without_spec", func(t *testing.T) {
		runSub(t, 2, "doctor")
	})

	t.Run("07_doctor_with_manifest_flag", func(t *testing.T) {
		runSub(t, 2, "doctor", "--spec", specPath, "--manifest", manifestPath)
	})

	t.Run("08_attach_missing_args", func(t *testing.T) {
		runSub(t, 2, "attach", "--spec", specPath, "--session", "smoke-sess-001")
	})

	t.Run("09_status_missing_file", func(t *testing.T) {
		runSub(t, 2, "status", "--manifest", filepath.Join(tempDir, "nonexistent.json"))
	})

	t.Run("10_doctor_invalid_spec_unknown_fields", func(t *testing.T) {
		badSpecPath := filepath.Join(tempDir, "bad-spec.json")
		_ = os.WriteFile(badSpecPath, []byte(strings.Replace(validSpecJSON, `"description":`, `"extraField": 123, "description":`, 1)), 0o644)
		runSub(t, 2, "doctor", "--spec", badSpecPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL)
	})

	t.Run("11_attach_invalid_binding_branch_mismatch", func(t *testing.T) {
		branchMismatchSpecPath := filepath.Join(tempDir, "branch-mismatch-spec.json")
		_ = os.WriteFile(branchMismatchSpecPath, []byte(strings.Replace(validSpecJSON, `"expectedBranch": "smoke-branch"`, `"expectedBranch": "wrong-branch"`, 1)), 0o644)
		runSub(t, 2, "attach", "--spec", branchMismatchSpecPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", filepath.Join(tempDir, "mismatch-manifest.json"), "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL)
	})

	t.Run("12_attach_invalid_lease_locked", func(t *testing.T) {
		leaseDir := filepath.Join(worktreeDir, ".agents-coworkers", "recovery-leases")
		_ = os.MkdirAll(leaseDir, 0o755)
		keyBytes := []byte("run-smoke-20261001-001\n__run__")
		keySum := sha256.Sum256(keyBytes)
		_ = os.WriteFile(filepath.Join(leaseDir, fmt.Sprintf("%x.lease", keySum)), []byte(`{"version": 1, "taskId": "__run__", "ownerId": "other-run", "pid": 9999}`), 0o644)
		runSub(t, 2, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", filepath.Join(tempDir, "locked-manifest.json"), "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL)
		_ = os.RemoveAll(leaseDir)
	})

	t.Run("13_doctor_gateway_auth_error_401", func(t *testing.T) {
		badAuthGW := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer badAuthGW.Close()
		runSub(t, 2, "doctor", "--spec", specPath, "--ao-url", aoServer.URL, "--gateway-url", badAuthGW.URL)
	})

	t.Run("14_doctor_closed_ao_port", func(t *testing.T) {
		runSub(t, 3, "doctor", "--spec", specPath, "--ao-url", "http://127.0.0.1:59999", "--gateway-url", gwServer.URL)
	})

	t.Run("15_doctor_model_missing_from_catalog", func(t *testing.T) {
		missingModelSpecPath := filepath.Join(tempDir, "missing-model-spec.json")
		_ = os.WriteFile(missingModelSpecPath, []byte(strings.Replace(validSpecJSON, `"model": "gpt-5"`, `"model": "nonexistent-model"`, 1)), 0o644)
		runSub(t, 2, "doctor", "--spec", missingModelSpecPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL)
	})

	t.Run("16_attach_conversation_missing_effort", func(t *testing.T) {
		badConvAO := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/identity" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-smoke-host", "apiVersion": 1})
				return
			}
			if r.URL.Path == "/api/v1/sessions/smoke-sess-001" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"session": map[string]any{
						"id":           "smoke-sess-001",
						"projectId":    "proj-smoke",
						"kind":         "worker",
						"harness":      "codex",
						"model":        "gpt-5",
						"branch":       "smoke-branch",
						"isTerminated": false,
					},
				})
				return
			}
			if r.URL.Path == "/api/v1/sessions/smoke-sess-001/conversation" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"settings": map[string]any{}})
				return
			}
			http.NotFound(w, r)
		}))
		defer badConvAO.Close()
		runSub(t, 2, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", filepath.Join(tempDir, "conv-err.json"), "--ao-url", badConvAO.URL, "--gateway-url", gwServer.URL)
	})

	t.Run("17_attach_profile_effort_mismatch", func(t *testing.T) {
		mismatchEffortAO := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/identity" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-smoke-host", "apiVersion": 1})
				return
			}
			if r.URL.Path == "/api/v1/sessions/smoke-sess-001" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"session": map[string]any{
						"id":           "smoke-sess-001",
						"projectId":    "proj-smoke",
						"kind":         "worker",
						"harness":      "codex",
						"model":        "gpt-5",
						"branch":       "smoke-branch",
						"isTerminated": false,
					},
				})
				return
			}
			if r.URL.Path == "/api/v1/sessions/smoke-sess-001/conversation" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"settings": map[string]any{"model": "gpt-5", "reasoningEffort": "high"}})
				return
			}
			http.NotFound(w, r)
		}))
		defer mismatchEffortAO.Close()
		runSub(t, 2, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", filepath.Join(tempDir, "effort-mismatch.json"), "--ao-url", mismatchEffortAO.URL, "--gateway-url", gwServer.URL)
	})

	t.Run("18_status_gateway_override_mismatch", func(t *testing.T) {
		runSub(t, 2, "status", "--manifest", manifestPath, "--ao-url", aoServer.URL, "--gateway-url", "http://127.0.0.1:8999")
	})

	t.Run("19_status_trailing_json_manifest", func(t *testing.T) {
		manifestContent, _ := os.ReadFile(manifestPath)
		trailingManifestPath := filepath.Join(tempDir, "trailing-manifest.json")
		_ = os.WriteFile(trailingManifestPath, append(manifestContent, []byte(` {"extra": 1}`)...), 0o644)
		runSub(t, 2, "status", "--manifest", trailingManifestPath)
	})

	t.Run("20_doctor_candidate_deduplication", func(t *testing.T) {
		mockHome := filepath.Join(tempDir, "mock-home-dedupe")
		mockAODir := filepath.Join(mockHome, ".ao")
		mockAODevDir := filepath.Join(mockAODir, "dev")
		_ = os.MkdirAll(mockAODevDir, 0o755)
		mockRunFile1 := filepath.Join(mockAODevDir, "running.json")
		mockRunFile2 := filepath.Join(mockAODir, "running.json")
		u, _ := http.NewRequest("GET", aoServer.URL, nil)
		portStr := u.URL.Port()
		var port int
		fmt.Sscanf(portStr, "%d", &port)
		runData := fmt.Sprintf(`{"pid": %d, "port": %d}`, os.Getpid(), port)
		_ = os.WriteFile(mockRunFile1, []byte(runData), 0o644)
		_ = os.WriteFile(mockRunFile2, []byte(runData), 0o644)
		t.Setenv("USERPROFILE", mockHome)
		t.Setenv("HOME", mockHome)
		t.Setenv("AO_BASE_URL", "")
		t.Setenv("AO_RUN_FILE", "")
		runSub(t, 0, "doctor", "--spec", specPath, "--gateway-url", gwServer.URL)
	})

	t.Run("21_doctor_candidate_invalid_exists", func(t *testing.T) {
		mockHome := filepath.Join(tempDir, "mock-home-invalid")
		mockAODir := filepath.Join(mockHome, ".ao")
		_ = os.MkdirAll(mockAODir, 0o755)
		mockRunFile := filepath.Join(mockAODir, "running.json")
		_ = os.WriteFile(mockRunFile, []byte(`{"pid": -1, "port": 0}`), 0o644)
		t.Setenv("USERPROFILE", mockHome)
		t.Setenv("HOME", mockHome)
		t.Setenv("AO_BASE_URL", "")
		t.Setenv("AO_RUN_FILE", "")
		runSub(t, 2, "doctor", "--spec", specPath, "--gateway-url", gwServer.URL)
	})

	t.Run("22_doctor_two_different_daemons_fail", func(t *testing.T) {
		mockHome := filepath.Join(tempDir, "mock-home-two-daemons")
		mockAODir := filepath.Join(mockHome, ".ao")
		mockAODevDir := filepath.Join(mockAODir, "dev")
		_ = os.MkdirAll(mockAODevDir, 0o755)
		mockRunFile1 := filepath.Join(mockAODevDir, "running.json")
		mockRunFile2 := filepath.Join(mockAODir, "running.json")

		u1, _ := http.NewRequest("GET", aoServer.URL, nil)
		portStr1 := u1.URL.Port()
		var port1 int
		fmt.Sscanf(portStr1, "%d", &port1)
		_ = os.WriteFile(mockRunFile1, []byte(fmt.Sprintf(`{"pid": %d, "port": %d}`, os.Getpid(), port1)), 0o644)

		aoServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/identity" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-second-daemon", "apiVersion": 1})
				return
			}
			http.NotFound(w, r)
		}))
		defer aoServer2.Close()

		u2, _ := http.NewRequest("GET", aoServer2.URL, nil)
		portStr2 := u2.URL.Port()
		var port2 int
		fmt.Sscanf(portStr2, "%d", &port2)
		_ = os.WriteFile(mockRunFile2, []byte(fmt.Sprintf(`{"pid": %d, "port": %d}`, os.Getpid(), port2)), 0o644)

		t.Setenv("USERPROFILE", mockHome)
		t.Setenv("HOME", mockHome)
		t.Setenv("AO_BASE_URL", "")
		t.Setenv("AO_RUN_FILE", "")
		runSub(t, 2, "doctor", "--spec", specPath, "--gateway-url", gwServer.URL)
	})
	t.Run("23_attach_idempotent_when_same_run_lease_owned", func(t *testing.T) {
		fl, err := recovery.NewFileLease(worktreeDir, "run-smoke-20261001-001")
		if err != nil {
			t.Fatalf("NewFileLease failed: %v", err)
		}
		release, err := fl.Acquire(context.Background(), "run-smoke-20261001-001", "__run__")
		if err != nil {
			t.Fatalf("fl.Acquire failed: %v", err)
		}
		defer release()

		fiBefore, err := os.Stat(manifestPath)
		if err != nil {
			t.Fatalf("stat manifestPath before: %v", err)
		}
		bytesBefore, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read manifestPath before: %v", err)
		}

		runSub(t, 0, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", manifestPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL, "--json")

		fiAfter, err := os.Stat(manifestPath)
		if err != nil {
			t.Fatalf("stat manifestPath after: %v", err)
		}
		bytesAfter, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("read manifestPath after: %v", err)
		}

		if !bytes.Equal(bytesBefore, bytesAfter) {
			t.Fatalf("expected manifest content bytes to be untouched across idempotent attach")
		}
		if !fiBefore.ModTime().Equal(fiAfter.ModTime()) {
			t.Fatalf("expected manifest file modification time to be untouched, before=%v, after=%v",
				fiBefore.ModTime(), fiAfter.ModTime())
		}
	})
	t.Run("24_doctor_and_attach_fail_when_product_root_has_agents_coworkers_file", func(t *testing.T) {
		unexpectedDir := filepath.Join(productRoot, ".agents-coworkers")
		_ = os.MkdirAll(unexpectedDir, 0o755)
		unexpectedFile := filepath.Join(unexpectedDir, "unexpected.txt")
		_ = os.WriteFile(unexpectedFile, []byte("bad"), 0o644)
		defer func() {
			_ = os.RemoveAll(unexpectedDir)
		}()

		runSub(t, 2, "doctor", "--spec", specPath, "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL)
		runSub(t, 2, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", filepath.Join(tempDir, "prod-dirty-manifest.json"), "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL)
	})

	t.Run("25_attach_fails_when_execution_workspace_has_unexpected_file", func(t *testing.T) {
		unexpectedDir := filepath.Join(worktreeDir, ".agents-coworkers")
		_ = os.MkdirAll(unexpectedDir, 0o755)
		unexpectedFile := filepath.Join(unexpectedDir, "unexpected.txt")
		_ = os.WriteFile(unexpectedFile, []byte("bad"), 0o644)
		defer func() {
			_ = os.RemoveAll(unexpectedDir)
		}()

		runSub(t, 2, "attach", "--spec", specPath, "--session", "smoke-sess-001", "--workspace", worktreeDir, "--manifest", filepath.Join(tempDir, "exec-dirty-manifest.json"), "--ao-url", aoServer.URL, "--gateway-url", gwServer.URL)
	})
}
