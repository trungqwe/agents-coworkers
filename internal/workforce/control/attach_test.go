package control

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

func createTestWorktree(t *testing.T) (string, string, string) {
	productRoot, commitSha := createTestGitRepo(t)
	execWorktree := filepath.Join(t.TempDir(), "worktree-sess-01")

	// Create worktree on branch feature/sess-01
	cmd := exec.Command("git", "-C", productRoot, "worktree", "add", "-b", "feature/sess-01", execWorktree, commitSha)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree add failed: %v, out: %s", err, string(out))
	}

	return productRoot, execWorktree, commitSha
}

func TestAttach_LifecycleAndIdempotent(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	// Fake AO Server
	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":           "sess-123",
					"projectId":    "proj-123",
					"kind":         "worker",
					"harness":      "codex",
					"model":        "gpt-5",
					"branch":       "feature/sess-01",
					"isTerminated": false,
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "gpt-5",
					"reasoningEffort": "high",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer aoServer.Close()

	// Fake Gateway Server
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	// Create RunSpec
	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-attach-001",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
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
      "reasoningEffort": "high"
    }
  }
}`, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)

	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-123",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
		JSON:         true,
	}

	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	// 1. Initial attach creates manifest
	manifest, exitCode := RunAttach(ctx, opts, &stdout, &stderr)
	if exitCode != ExitCodeSuccess {
		t.Fatalf("RunAttach failed with code %d; stderr: %s", exitCode, stderr.String())
	}
	if manifest == nil || manifest.RunID != "run-attach-001" {
		t.Fatalf("unexpected manifest returned: %+v", manifest)
	}
	if manifest.WorktreeBinding.WorktreeBranch != "feature/sess-01" {
		t.Errorf("unexpected worktree branch: %s", manifest.WorktreeBinding.WorktreeBranch)
	}

	// 2. Second attach is idempotent success
	stdout.Reset()
	stderr.Reset()
	manifest2, exitCode2 := RunAttach(ctx, opts, &stdout, &stderr)
	if exitCode2 != ExitCodeSuccess {
		t.Fatalf("idempotent RunAttach failed with code %d; stderr: %s", exitCode2, stderr.String())
	}
	if manifest2.RunID != manifest.RunID {
		t.Errorf("expected same manifest on idempotent run")
	}
}

func TestAttach_RequireCleanFailsOnDirtyWorktree(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	// Make worktree dirty
	dirtyFile := filepath.Join(execWorktree, "uncommitted.txt")
	_ = os.WriteFile(dirtyFile, []byte("dirty"), 0o644)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":        "sess-123",
					"projectId": "proj-123",
					"kind":      "worker",
					"harness":   "codex",
					"model":     "gpt-5",
					"branch":    "feature/sess-01",
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "gpt-5",
					"reasoningEffort": "high",
				},
			})
			return
		}
	}))
	defer aoServer.Close()
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-attach-dirty",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
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
      "reasoningEffort": "high"
    }
  }
}`, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)
	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-123",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on dirty worktree with require_clean, got %d", exitCode)
	}
}

func TestAttach_AllowDirtyRecordedPasses(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	// Make worktree dirty
	dirtyFile := filepath.Join(execWorktree, "uncommitted.txt")
	_ = os.WriteFile(dirtyFile, []byte("dirty"), 0o644)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":        "sess-123",
					"projectId": "proj-123",
					"kind":      "worker",
					"harness":   "codex",
					"model":     "gpt-5",
					"branch":    "feature/sess-01",
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "gpt-5",
					"reasoningEffort": "medium",
				},
			})
			return
		}
	}))
	defer aoServer.Close()
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-attach-allow-dirty",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "allow_dirty_recorded"
  },
  "requiredTools": [{"name": "git", "required": true}],
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
}`, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)
	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-123",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	manifest, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeSuccess {
		t.Fatalf("expected ExitCodeSuccess on allow_dirty_recorded, got %d; stderr: %s", exitCode, stderr.String())
	}
	if manifest.WorktreeBinding.IsClean {
		t.Error("expected IsClean false")
	}
	if manifest.WorktreeBinding.PorcelainSha256 == "" {
		t.Error("expected non-empty porcelainSha256")
	}
}

func TestAttach_ConversationReadbackFailureFails(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":        "sess-123",
					"projectId": "proj-123",
					"kind":      "worker",
					"harness":   "codex",
					"model":     "gpt-5",
					"branch":    "feature/sess-01",
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123/conversation" {
			// Conversation readback returns 500 error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}))
	defer aoServer.Close()
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-attach-conv-err",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
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
      "reasoningEffort": "high"
    }
  }
}`, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)
	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-123",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on conversation readback failure, got %d", exitCode)
	}
}

func TestAttach_ConversationModelEmptyOrMismatch(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	// 1. Conversation returns empty model -> exit 2
	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":        "sess-123",
					"projectId": "proj-123",
					"kind":      "worker",
					"harness":   "codex",
					"model":     "gpt-5",
					"branch":    "feature/sess-01",
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "", // empty model
					"reasoningEffort": "high",
				},
			})
			return
		}
	}))
	defer aoServer.Close()
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-attach-conv-model-empty",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
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
      "reasoningEffort": "high"
    }
  }
}`, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)
	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-123",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on empty conversation model, got %d", exitCode)
	}
}

func TestAttach_ModelNotInGatewayCatalog(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":        "sess-123",
					"projectId": "proj-123",
					"kind":      "worker",
					"harness":   "codex",
					"model":     "gpt-5",
					"branch":    "feature/sess-01",
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "gpt-5",
					"reasoningEffort": "high",
				},
			})
			return
		}
	}))
	defer aoServer.Close()

	// Gateway does NOT have gpt-5
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "other-model"}}})
	}))
	defer gwServer.Close()

	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-attach-not-in-catalog",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
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
      "reasoningEffort": "high"
    }
  }
}`, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)
	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-123",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) when session model is not in gateway catalog, got %d", exitCode)
	}
}

func TestAttach_ProfileModelMismatchFails(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-123" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":        "sess-123",
					"projectId": "proj-123",
					"kind":      "worker",
					"harness":   "codex",
					"model":     "gpt-5", // Session has gpt-5
					"branch":    "feature/sess-01",
				},
			})
			return
		}
	}))
	defer aoServer.Close()
	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	// Spec declares worker profile with "claude-3"
	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-attach-model-mismatch",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
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
}`, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)
	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-123",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
	}

	var stdout, stderr bytes.Buffer
	_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on model mismatch, got %d", exitCode)
	}
}

func TestAttach_IdempotentWhenLeaseTransitionedFreeToOwned(t *testing.T) {
	productRoot, execWorktree, commitSha := createTestWorktree(t)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-lease-free-owned" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":           "sess-lease-free-owned",
					"projectId":    "proj-123",
					"kind":         "worker",
					"harness":      "codex",
					"model":        "gpt-5",
					"branch":       "feature/sess-01",
					"isTerminated": false,
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-lease-free-owned/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "gpt-5",
					"reasoningEffort": "high",
				},
			})
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

	runID := "run-lease-free-to-owned-001"
	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "%s",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
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
      "reasoningEffort": "high"
    }
  }
}`, runID, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)

	manifestFile := filepath.Join(t.TempDir(), "run-manifest.json")

	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-lease-free-owned",
		Workspace:    execWorktree,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
		JSON:         true,
	}

	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	// 1. Initial attach creates manifest with lease FREE and observedPid 0
	manifest1, exitCode1 := RunAttach(ctx, opts, &stdout, &stderr)
	if exitCode1 != ExitCodeSuccess {
		t.Fatalf("initial RunAttach failed with code %d; stderr: %s", exitCode1, stderr.String())
	}
	if manifest1.Lease.ObservedState != recovery.LeaseStateFree || manifest1.Lease.ObservedPID != 0 {
		t.Fatalf("expected initial lease to be FREE/0, got state=%s, pid=%d",
			manifest1.Lease.ObservedState, manifest1.Lease.ObservedPID)
	}

	stat1, err := os.Stat(manifestFile)
	if err != nil {
		t.Fatalf("stat manifestFile after first attach: %v", err)
	}
	bytes1, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatalf("read manifestFile after first attach: %v", err)
	}

	// 2. Same run acquires lease, transitioning lease on disk to OWNED with observedPid > 0
	fl, err := recovery.NewFileLease(execWorktree, runID)
	if err != nil {
		t.Fatalf("NewFileLease failed: %v", err)
	}
	release, err := fl.Acquire(ctx, runID, "__run__")
	if err != nil {
		t.Fatalf("fl.Acquire failed: %v", err)
	}
	defer release()

	insp, err := recovery.InspectLease(execWorktree, runID, "__run__", runID)
	if err != nil {
		t.Fatalf("InspectLease failed: %v", err)
	}
	if insp.ObservedState != recovery.LeaseStateOwned || insp.ObservedPID <= 0 {
		t.Fatalf("expected lease state OWNED with PID > 0, got state=%s, pid=%d", insp.ObservedState, insp.ObservedPID)
	}

	// 3. Re-attach same run/spec/session/repository/worktree
	stdout.Reset()
	stderr.Reset()
	manifest2, exitCode2 := RunAttach(ctx, opts, &stdout, &stderr)
	if exitCode2 != ExitCodeSuccess {
		t.Fatalf("re-attach failed with code %d; stderr: %s", exitCode2, stderr.String())
	}
	if manifest2.RunID != manifest1.RunID {
		t.Fatalf("re-attach returned different runId: %s vs %s", manifest2.RunID, manifest1.RunID)
	}

	// 4. Verify timestamp and bytes of manifest file on disk are untouched
	stat2, err := os.Stat(manifestFile)
	if err != nil {
		t.Fatalf("stat manifestFile after re-attach: %v", err)
	}
	bytes2, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatalf("read manifestFile after re-attach: %v", err)
	}

	if !bytes.Equal(bytes1, bytes2) {
		t.Fatalf("manifest bytes changed across re-attach!\nbefore:\n%s\nafter:\n%s", string(bytes1), string(bytes2))
	}
	if !stat1.ModTime().Equal(stat2.ModTime()) {
		t.Fatalf("manifest ModTime changed across re-attach: before=%v, after=%v", stat1.ModTime(), stat2.ModTime())
	}
}

func TestAttach_DirtyPolicyStrictness_CasesA_B_C_D(t *testing.T) {
	setupServers := func() (*httptest.Server, *httptest.Server) {
		ao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/identity" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
				return
			}
			if r.URL.Path == "/api/v1/sessions/sess-dirty-test" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"session": map[string]any{
						"id":           "sess-dirty-test",
						"projectId":    "proj-123",
						"kind":         "worker",
						"harness":      "codex",
						"model":        "gpt-5",
						"branch":       "feature/sess-01",
						"isTerminated": false,
					},
				})
				return
			}
			if r.URL.Path == "/api/v1/sessions/sess-dirty-test/conversation" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"settings": map[string]any{
						"model":           "gpt-5",
						"reasoningEffort": "high",
					},
				})
				return
			}
			http.NotFound(w, r)
		}))
		gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
		}))
		return ao, gw
	}

	aoServer, gwServer := setupServers()
	defer aoServer.Close()
	defer gwServer.Close()

	runID := "run-strict-dirty-001"

	createSpec := func(productRoot, execWorktree, commitSha, dirtyPolicy string) string {
		specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "%s",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "%s"
  },
  "requiredTools": [{"name": "git", "required": true}],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "profiles": {
    "worker": {
      "model": "gpt-5",
      "reasoningEffort": "high"
    }
  }
}`, runID, filepath.ToSlash(productRoot), filepath.ToSlash(execWorktree), commitSha, dirtyPolicy)
		specFile := filepath.Join(t.TempDir(), "run-spec.json")
		_ = os.WriteFile(specFile, []byte(specContent), 0o644)
		return specFile
	}

	// Case A: Product root has .agents-coworkers/unexpected.txt -> attach exit 2
	t.Run("Case_A_ProductRootHasAgentsCoworkersUnexpectedFile", func(t *testing.T) {
		prodRoot, execWT, commitSha := createTestWorktree(t)
		unexpectedDir := filepath.Join(prodRoot, ".agents-coworkers")
		_ = os.MkdirAll(unexpectedDir, 0o755)
		_ = os.WriteFile(filepath.Join(unexpectedDir, "unexpected.txt"), []byte("bad"), 0o644)

		specFile := createSpec(prodRoot, execWT, commitSha, "require_clean")
		manifestFile := filepath.Join(t.TempDir(), "manifest.json")
		opts := &AttachOptions{
			SpecPath:     specFile,
			SessionID:    "sess-dirty-test",
			Workspace:    execWT,
			ManifestPath: manifestFile,
			AOURL:        aoServer.URL,
			GatewayURL:   gwServer.URL,
		}
		var stdout, stderr bytes.Buffer
		_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
		if exitCode != ExitCodeValidation {
			t.Fatalf("expected ExitCodeValidation (2) when product root has unexpected file, got %d", exitCode)
		}
	})

	// Case B: Execution workspace has .agents-coworkers/unexpected.txt -> require_clean exit 2
	t.Run("Case_B_ExecutionWorkspaceHasUnexpectedFile", func(t *testing.T) {
		prodRoot, execWT, commitSha := createTestWorktree(t)
		unexpectedDir := filepath.Join(execWT, ".agents-coworkers")
		_ = os.MkdirAll(unexpectedDir, 0o755)
		_ = os.WriteFile(filepath.Join(unexpectedDir, "unexpected.txt"), []byte("bad"), 0o644)

		specFile := createSpec(prodRoot, execWT, commitSha, "require_clean")
		manifestFile := filepath.Join(t.TempDir(), "manifest.json")
		opts := &AttachOptions{
			SpecPath:     specFile,
			SessionID:    "sess-dirty-test",
			Workspace:    execWT,
			ManifestPath: manifestFile,
			AOURL:        aoServer.URL,
			GatewayURL:   gwServer.URL,
		}
		var stdout, stderr bytes.Buffer
		_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
		if exitCode != ExitCodeValidation {
			t.Fatalf("expected ExitCodeValidation (2) when execution workspace has unexpected file, got %d", exitCode)
		}
	})

	// Case C: Execution workspace has lease file of other run -> require_clean exit 2
	t.Run("Case_C_ExecutionWorkspaceHasOtherRunLeaseFile", func(t *testing.T) {
		prodRoot, execWT, commitSha := createTestWorktree(t)
		leaseDir := filepath.Join(execWT, ".agents-coworkers", "recovery-leases")
		_ = os.MkdirAll(leaseDir, 0o755)
		otherKey := sha256.Sum256([]byte("other-run-id\n__run__"))
		_ = os.WriteFile(filepath.Join(leaseDir, fmt.Sprintf("%x.lease", otherKey)), []byte(`{"pid": 1111}`), 0o644)

		specFile := createSpec(prodRoot, execWT, commitSha, "require_clean")
		manifestFile := filepath.Join(t.TempDir(), "manifest.json")
		opts := &AttachOptions{
			SpecPath:     specFile,
			SessionID:    "sess-dirty-test",
			Workspace:    execWT,
			ManifestPath: manifestFile,
			AOURL:        aoServer.URL,
			GatewayURL:   gwServer.URL,
		}
		var stdout, stderr bytes.Buffer
		_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
		if exitCode != ExitCodeValidation {
			t.Fatalf("expected ExitCodeValidation (2) when execution workspace has other run lease, got %d", exitCode)
		}
	})

	// Case D: Execution workspace has non-lease file in recovery-leases/ -> require_clean exit 2
	t.Run("Case_D_ExecutionWorkspaceHasNonLeaseFileInLeaseDir", func(t *testing.T) {
		prodRoot, execWT, commitSha := createTestWorktree(t)
		leaseDir := filepath.Join(execWT, ".agents-coworkers", "recovery-leases")
		_ = os.MkdirAll(leaseDir, 0o755)
		_ = os.WriteFile(filepath.Join(leaseDir, "garbage.txt"), []byte("not a lease"), 0o644)

		specFile := createSpec(prodRoot, execWT, commitSha, "require_clean")
		manifestFile := filepath.Join(t.TempDir(), "manifest.json")
		opts := &AttachOptions{
			SpecPath:     specFile,
			SessionID:    "sess-dirty-test",
			Workspace:    execWT,
			ManifestPath: manifestFile,
			AOURL:        aoServer.URL,
			GatewayURL:   gwServer.URL,
		}
		var stdout, stderr bytes.Buffer
		_, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
		if exitCode != ExitCodeValidation {
			t.Fatalf("expected ExitCodeValidation (2) when execution workspace has non-lease file in recovery-leases, got %d", exitCode)
		}
	})
}

// Case F: allow_dirty_recorded
func TestAttach_CaseF_AllowDirtyRecorded_ExcludesExactLeaseAndHashesRemainingEntries(t *testing.T) {
	prodRoot, execWT, commitSha := createTestWorktree(t)

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-dirty-f" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{
					"id":           "sess-dirty-f",
					"projectId":    "proj-123",
					"kind":         "worker",
					"harness":      "codex",
					"model":        "gpt-5",
					"branch":       "feature/sess-01",
					"isTerminated": false,
				},
			})
			return
		}
		if r.URL.Path == "/api/v1/sessions/sess-dirty-f/conversation" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"settings": map[string]any{
					"model":           "gpt-5",
					"reasoningEffort": "high",
				},
			})
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

	runID := "run-case-f-001"
	specContent := fmt.Sprintf(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "%s",
  "target": {
    "targetRoot": "%s",
    "executionWorkspace": "%s",
    "baselineSha": "%s",
    "expectedBranch": "feature/sess-01"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "allow_dirty_recorded"
  },
  "requiredTools": [{"name": "git", "required": true}],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "profiles": {
    "worker": {
      "model": "gpt-5",
      "reasoningEffort": "high"
    }
  }
}`, runID, filepath.ToSlash(prodRoot), filepath.ToSlash(execWT), commitSha)

	specFile := filepath.Join(t.TempDir(), "run-spec.json")
	_ = os.WriteFile(specFile, []byte(specContent), 0o644)

	// 1. Create exact same-run lease file
	fl, err := recovery.NewFileLease(execWT, runID)
	if err != nil {
		t.Fatalf("NewFileLease failed: %v", err)
	}
	release, err := fl.Acquire(context.Background(), runID, "__run__")
	if err != nil {
		t.Fatalf("fl.Acquire failed: %v", err)
	}
	defer release()

	// 2. Create another dirty source file
	dirtyFile := filepath.Join(execWT, "dirty_source_file.txt")
	_ = os.WriteFile(dirtyFile, []byte("uncommitted code change"), 0o644)

	// 3. Compute independent oracle hash:
	// Run git status --porcelain=v1 -uall -z with pathspec dirty_source_file.txt directly.
	// Does NOT call any production helper or filter.
	pathspecCmd := exec.Command("git", "-C", execWT, "status", "--porcelain=v1", "-uall", "-z", "--", "dirty_source_file.txt")
	pathspecOut, err := pathspecCmd.Output()
	if err != nil {
		t.Fatalf("git status with pathspec failed: %v", err)
	}
	if len(pathspecOut) == 0 {
		t.Fatalf("expected pathspec output not to be empty")
	}
	if !bytes.Contains(pathspecOut, []byte("dirty_source_file.txt")) {
		t.Fatalf("expected pathspec output to contain dirty_source_file.txt, got %q", string(pathspecOut))
	}
	key := sha256.Sum256([]byte(runID + "\n__run__"))
	expectedLeaseRel := fmt.Sprintf(".agents-coworkers/recovery-leases/%x.lease", key)
	if bytes.Contains(pathspecOut, []byte(expectedLeaseRel)) {
		t.Fatalf("expected pathspec output not to contain lease record, got %q", string(pathspecOut))
	}

	// Confirm full raw porcelain still contains exact lease record
	rawCmd := exec.Command("git", "-C", execWT, "status", "--porcelain=v1", "-uall", "-z")
	rawOut, err := rawCmd.Output()
	if err != nil {
		t.Fatalf("git status raw failed: %v", err)
	}
	if !bytes.Contains(rawOut, []byte(expectedLeaseRel)) {
		t.Fatalf("expected full raw porcelain to contain exact lease record %s", expectedLeaseRel)
	}

	oracleSum := sha256.Sum256(pathspecOut)
	oracleHex := hex.EncodeToString(oracleSum[:])

	// 4. Run attach
	manifestFile := filepath.Join(t.TempDir(), "manifest.json")
	opts := &AttachOptions{
		SpecPath:     specFile,
		SessionID:    "sess-dirty-f",
		Workspace:    execWT,
		ManifestPath: manifestFile,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
		JSON:         true,
	}
	var stdout, stderr bytes.Buffer
	manifest, exitCode := RunAttach(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeSuccess {
		t.Fatalf("RunAttach failed with code %d; stderr: %s", exitCode, stderr.String())
	}

	if manifest.WorktreeBinding.IsClean {
		t.Errorf("expected isClean to be false")
	}
	if manifest.WorktreeBinding.PorcelainSha256 != oracleHex {
		t.Errorf("porcelainSha256 mismatch:\nexpected (oracle): %s\ngot: %s",
			oracleHex, manifest.WorktreeBinding.PorcelainSha256)
	}

	// Verify that rawOut hash differs from oracleHex (exact same-run lease did NOT go into the hash)
	rawSum := sha256.Sum256(rawOut)
	rawHex := hex.EncodeToString(rawSum[:])
	if rawHex == oracleHex {
		t.Errorf("expected raw porcelain hash to differ from filtered oracle hash (same-run lease was supposed to be excluded)")
	}
}
