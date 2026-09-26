package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func createTestGitRepo(t *testing.T) (string, string) {
	tmpDir := t.TempDir()
	initCmd := exec.Command("git", "init", tmpDir)
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	// config user
	_ = exec.Command("git", "-C", tmpDir, "config", "user.name", "Tester").Run()
	_ = exec.Command("git", "-C", tmpDir, "config", "user.email", "tester@example.com").Run()

	readme := filepath.Join(tmpDir, "README.md")
	_ = os.WriteFile(readme, []byte("# Test Repo\n"), 0o644)
	_ = exec.Command("git", "-C", tmpDir, "add", ".").Run()
	_ = exec.Command("git", "-C", tmpDir, "commit", "-m", "initial commit").Run()

	out, err := exec.Command("git", "-C", tmpDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD failed: %v", err)
	}
	commitSha := string(bytes.TrimSpace(out))
	return tmpDir, commitSha
}

func TestDoctor_Flags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// 1. Missing --spec -> error
	_, code, err := ParseDoctorFlags([]string{}, &stdout, &stderr)
	if code != ExitCodeValidation || err == nil {
		t.Errorf("expected ExitCodeValidation and error on missing --spec, got %d, %v", code, err)
	}

	// 2. Passing --manifest -> error
	_, code, err = ParseDoctorFlags([]string{"--spec", "spec.json", "--manifest", "out.json"}, &stdout, &stderr)
	if code != ExitCodeValidation || err == nil {
		t.Errorf("expected ExitCodeValidation on passing --manifest, got %d, %v", code, err)
	}

	// 3. Valid flags
	opts, code, err := ParseDoctorFlags([]string{"--spec", "spec.json", "--ao-url", "http://127.0.0.1:3000", "--json"}, &stdout, &stderr)
	if code != ExitCodeSuccess || err != nil {
		t.Fatalf("expected success on valid flags, got %d, %v", code, err)
	}
	if opts.SpecPath != "spec.json" || opts.AOURL != "http://127.0.0.1:3000" || !opts.JSON {
		t.Errorf("unexpected parsed opts: %+v", opts)
	}
}

func TestDoctor_RunPass(t *testing.T) {
	repoDir, commitSha := createTestGitRepo(t)

	// Fake AO server
	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		http.NotFound(w, r)
	}))
	defer aoServer.Close()

	// Fake Gateway server
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "gpt-5"}, {"id": "claude-3"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer gwServer.Close()

	// Write RunSpec
	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-doctor-001",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "main"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [
    {
      "name": "git",
      "required": true
    }
  ],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "profiles": {
    "worker": {
      "model": "gpt-5",
      "reasoningEffort": "medium"
    }
  }
}`, filepath.ToSlash(repoDir), filepath.ToSlash(repoDir), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)

	opts := &DoctorOptions{
		SpecPath:   specFile,
		AOURL:      aoServer.URL,
		GatewayURL: gwServer.URL,
		JSON:       true,
	}

	var stdout, stderr bytes.Buffer
	ctx := context.Background()
	report, exitCode := RunDoctor(ctx, opts, &stdout, &stderr)
	if exitCode != ExitCodeSuccess {
		t.Fatalf("RunDoctor failed with exit code %d; stderr: %s", exitCode, stderr.String())
	}
	if report.Status != "PASS" {
		t.Fatalf("expected report status PASS, got %s", report.Status)
	}
	if !report.TargetRootClean || !report.BaselineFound {
		t.Fatalf("expected TargetRootClean and BaselineFound true, got clean=%v, baseline=%v", report.TargetRootClean, report.BaselineFound)
	}
	if !report.ModelPresence["gpt-5"] {
		t.Errorf("expected model presence for gpt-5 to be true")
	}
	if report.GatewayEndpoint.ProviderCallPerformed != false || report.GatewayEndpoint.CredentialEligibility != "NOT_OBSERVED" {
		t.Errorf("unexpected gateway probe contract fields: %+v", report.GatewayEndpoint)
	}
}

func TestDoctor_MissingModelFails(t *testing.T) {
	repoDir, commitSha := createTestGitRepo(t)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
	}))
	defer aoServer.Close()

	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Gateway only has gpt-5, but spec requires claude-3
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "gpt-5"}},
		})
	}))
	defer gwServer.Close()

	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-doctor-missing-model",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "main"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [{"name": "git", "required": true}],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "profiles": {
    "worker": {
      "model": "claude-3",
      "reasoningEffort": "high"
    }
  }
}`, filepath.ToSlash(repoDir), filepath.ToSlash(repoDir), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)

	opts := &DoctorOptions{
		SpecPath:   specFile,
		AOURL:      aoServer.URL,
		GatewayURL: gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	report, exitCode := RunDoctor(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on missing model, got %d", exitCode)
	}
	if report.Status != "FAIL" {
		t.Errorf("expected report status FAIL, got %s", report.Status)
	}
}

func TestDoctor_DirtyWorkspacePolicy(t *testing.T) {
	repoDir, commitSha := createTestGitRepo(t)

	// Create a dirty file in repoDir
	dirtyFile := filepath.Join(repoDir, "dirty.txt")
	_ = os.WriteFile(dirtyFile, []byte("dirty content"), 0o644)

	// Fake AO and Gateway
	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
	}))
	defer aoServer.Close()
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "gpt-5"}},
		})
	}))
	defer gwServer.Close()

	// 1. TargetRoot dirty fails closed exit 2
	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-doctor-002",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "main"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [{"name": "git", "required": true}],
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
}`, filepath.ToSlash(repoDir), filepath.ToSlash(repoDir), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)

	opts := &DoctorOptions{
		SpecPath:   specFile,
		AOURL:      aoServer.URL,
		GatewayURL: gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	_, exitCode := RunDoctor(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) when product root is dirty, got %d", exitCode)
	}
}

func TestDoctor_FailsWhenProductRootHasAgentsCoworkersUnexpectedFile(t *testing.T) {
	repoDir, commitSha := createTestGitRepo(t)

	// Add unexpected file under .agents-coworkers in product root
	unexpectedDir := filepath.Join(repoDir, ".agents-coworkers")
	_ = os.MkdirAll(unexpectedDir, 0o755)
	_ = os.WriteFile(filepath.Join(unexpectedDir, "unexpected.txt"), []byte("intruder"), 0o644)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		http.NotFound(w, r)
	}))
	defer aoServer.Close()

	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-doc-unexpected-01",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "main"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [{"name": "git", "required": true}],
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
}`, filepath.ToSlash(repoDir), filepath.ToSlash(repoDir), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)

	opts := &DoctorOptions{
		SpecPath:   specFile,
		AOURL:      aoServer.URL,
		GatewayURL: gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	_, exitCode := RunDoctor(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) when product root has .agents-coworkers file, got %d", exitCode)
	}
}

func TestFilterExactLeaseArtifact_StatusMatrix(t *testing.T) {
	exactPath := ".agents-coworkers/recovery-leases/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.lease"

	// 1. `?? exact-path` is filtered out
	rawUntracked := []byte("?? " + exactPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawUntracked, exactPath); len(filtered) != 0 {
		t.Fatalf("expected '?? exact-path' to be filtered out, got %q", string(filtered))
	}

	// 2. `A  exact-path` is kept
	rawStaged := []byte("A  " + exactPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawStaged, exactPath); !bytes.Equal(filtered, rawStaged) {
		t.Fatalf("expected 'A  exact-path' to be kept, got %q", string(filtered))
	}

	// 3. ` M exact-path` is kept
	rawModifiedWorktree := []byte(" M " + exactPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawModifiedWorktree, exactPath); !bytes.Equal(filtered, rawModifiedWorktree) {
		t.Fatalf("expected ' M exact-path' to be kept, got %q", string(filtered))
	}

	// `M  exact-path` is kept
	rawModifiedIndex := []byte("M  " + exactPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawModifiedIndex, exactPath); !bytes.Equal(filtered, rawModifiedIndex) {
		t.Fatalf("expected 'M  exact-path' to be kept, got %q", string(filtered))
	}

	// 4. ` D exact-path` is kept
	rawDeletedWorktree := []byte(" D " + exactPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawDeletedWorktree, exactPath); !bytes.Equal(filtered, rawDeletedWorktree) {
		t.Fatalf("expected ' D exact-path' to be kept, got %q", string(filtered))
	}

	// `D  exact-path` is kept
	rawDeletedIndex := []byte("D  " + exactPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawDeletedIndex, exactPath); !bytes.Equal(filtered, rawDeletedIndex) {
		t.Fatalf("expected 'D  exact-path' to be kept, got %q", string(filtered))
	}

	// 5. rename/copy containing exact path is kept (both source and destination)
	rawRenameSrc := []byte("R  " + exactPath + "\x00destination.lease\x00")
	if filtered := filterExactLeaseArtifact(rawRenameSrc, exactPath); !bytes.Equal(filtered, rawRenameSrc) {
		t.Fatalf("expected rename with exact path as source to be kept, got %q", string(filtered))
	}

	rawRenameDest := []byte("R  source.lease\x00" + exactPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawRenameDest, exactPath); !bytes.Equal(filtered, rawRenameDest) {
		t.Fatalf("expected rename with exact path as destination to be kept, got %q", string(filtered))
	}

	// 6. path similar or other lease is kept
	otherLease := ".agents-coworkers/recovery-leases/ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff.lease"
	rawOther := []byte("?? " + otherLease + "\x00")
	if filtered := filterExactLeaseArtifact(rawOther, exactPath); !bytes.Equal(filtered, rawOther) {
		t.Fatalf("expected other lease to be kept, got %q", string(filtered))
	}

	similarPath := exactPath + ".tmp"
	rawSimilar := []byte("?? " + similarPath + "\x00")
	if filtered := filterExactLeaseArtifact(rawSimilar, exactPath); !bytes.Equal(filtered, rawSimilar) {
		t.Fatalf("expected similar path to be kept, got %q", string(filtered))
	}
}
