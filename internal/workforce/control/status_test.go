package control

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStatus_Flags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// Missing --manifest -> error
	_, code, err := ParseStatusFlags([]string{}, &stdout, &stderr)
	if code != ExitCodeValidation || err == nil {
		t.Fatalf("expected ExitCodeValidation on missing --manifest, got %d, %v", code, err)
	}

	// Valid flags
	opts, code, err := ParseStatusFlags([]string{"--manifest", "manifest.json", "--json"}, &stdout, &stderr)
	if code != ExitCodeSuccess || err != nil {
		t.Fatalf("expected ExitCodeSuccess on valid flags, got %d, %v", code, err)
	}
	if opts.ManifestPath != "manifest.json" || !opts.JSON {
		t.Errorf("unexpected parsed options: %+v", opts)
	}
}

func TestStatus_RunPass(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "run-manifest.json")

	m := sampleManifest("run-status-001", tmpDir)
	m.Lease.WorkspaceRoot = tmpDir
	m.Target.ExecutionWorkspace = tmpDir
	m.WorktreeBinding.CanonicalPath = tmpDir

	if err := WriteManifestAtomic(manifestPath, m); err != nil {
		t.Fatalf("WriteManifestAtomic failed: %v", err)
	}

	opts := &StatusOptions{
		ManifestPath: manifestPath,
		JSON:         true,
	}

	var stdout, stderr bytes.Buffer
	ctx := context.Background()
	report, exitCode := RunStatus(ctx, opts, &stdout, &stderr)
	if exitCode != ExitCodeSuccess {
		t.Fatalf("RunStatus failed with code %d; stderr: %s", exitCode, stderr.String())
	}
	if report == nil || report.RunID != "run-status-001" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.LiveLease.ObservedState != "FREE" {
		t.Errorf("expected live lease state FREE, got %s", report.LiveLease.ObservedState)
	}
}

func TestStatus_MissingOrCorruptManifest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := context.Background()

	// Missing manifest -> exit 2
	opts := &StatusOptions{ManifestPath: "/nonexistent/manifest.json"}
	_, exitCode := RunStatus(ctx, opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on missing manifest, got %d", exitCode)
	}

	// Corrupt manifest -> exit 2
	tmpDir := t.TempDir()
	badManifest := filepath.Join(tmpDir, "bad.json")
	_ = os.WriteFile(badManifest, []byte("bad-json"), 0o644)
	opts = &StatusOptions{ManifestPath: badManifest}
	_, exitCode = RunStatus(ctx, opts, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on corrupt manifest, got %d", exitCode)
	}
}

func TestStatus_EndpointOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "run-manifest.json")

	aoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-host-01", "apiVersion": 1})
	}))
	defer aoServer.Close()

	gwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "gpt-5"}}})
	}))
	defer gwServer.Close()

	m := sampleManifest("run-status-overrides", tmpDir)
	m.Lease.WorkspaceRoot = tmpDir
	m.Target.ExecutionWorkspace = tmpDir
	m.WorktreeBinding.CanonicalPath = tmpDir
	m.Endpoints.AOURL = aoServer.URL
	m.Endpoints.AOIdentity = "ao-host-01"
	m.Endpoints.GatewayProbe.URL = gwServer.URL

	if err := WriteManifestAtomic(manifestPath, m); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}

	// 1. Both overrides match -> success
	opts := &StatusOptions{
		ManifestPath: manifestPath,
		AOURL:        aoServer.URL,
		GatewayURL:   gwServer.URL,
	}
	var stdout, stderr bytes.Buffer
	_, exitCode := RunStatus(context.Background(), opts, &stdout, &stderr)
	if exitCode != ExitCodeSuccess {
		t.Fatalf("expected success with matching overrides, got %d, stderr: %s", exitCode, stderr.String())
	}

	// 2. Gateway override mismatch with manifest bound URL -> exit 2
	optsMismatchGW := &StatusOptions{
		ManifestPath: manifestPath,
		GatewayURL:   "http://127.0.0.1:8999", // Different URL than bound
	}
	stdout.Reset()
	stderr.Reset()
	_, exitCode = RunStatus(context.Background(), optsMismatchGW, &stdout, &stderr)
	if exitCode != ExitCodeValidation {
		t.Fatalf("expected ExitCodeValidation (2) on gateway override mismatch, got %d", exitCode)
	}
}
